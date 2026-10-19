// gocpp/waiter.hpp: gp::waiter, how a goroutine blocks.
//
// Every blocking operation of the runtime (a channel send or receive, select, sync's Mutex,
// WaitGroup and Cond, time.Sleep) parks the goroutine's thread on a waiter, and whoever makes
// the operation possible unparks it (ADR-007). It is Go's gopark and goready, with the OS
// thread as the goroutine: a mutex and a condition variable of the C++ library (so it builds
// on every OS the runtime does; the Windows port needs nothing here). They are in src/waiter.cpp,
// not in this header: gocpp/rt.hpp includes it, and <mutex>, <condition_variable> and <chrono>
// would add a second to the compile of every generated file (and their macros to the names the
// emitter must avoid). So a waiter is a pointer to its state, and the timed calls are templates
// that need no <chrono> (any duration or time point of the C++ library works).
//
// A waiter holds one permit, as Rust's thread::park does:
//   - gp_unpark makes the permit available and wakes the thread that is parked. Several unparks
//     before a park are one permit (they don't add up).
//   - gp_park returns when a permit is there, and takes it: at once when gp_unpark came first
//     (unpark before park is not lost), after a wait otherwise. The C++ library's own spurious
//     wake-ups never get out of it.
// But a permit can be older than what the parked thread waits for: an unpark that raced with
// an earlier wait is still there at the next park. So a caller parks in a loop on its own
// condition (the way a condition variable is used), and the one who unparks makes the
// condition true first:
//
//     while (!ready.load(std::memory_order_acquire)) w.gp_park();    // the parked goroutine
//     ready.store(true, std::memory_order_release); w.gp_unpark();   // the one that wakes it
//
// One thread parks on a waiter at a time (a waiter belongs to a goroutine, which is one
// thread); any number of threads may unpark it. gp_unpark is the last thing a waker may do with
// a waiter that the parked thread owns: the owner may destroy it as soon as gp_park returns,
// and gp_unpark works under the waiter's lock, so it never touches the waiter after the owner
// could get out of gp_park.
//
// gp_unpark_with(f) and gp_wait(pred) are for the one who wakes up a parked goroutine and tells
// it that its operation is done (a channel hands a value over and says "done" in a cell on the
// parked goroutine's stack): f runs under the waiter's lock, then the permit is made and the
// thread is woken; pred is evaluated under the same lock, until it is true. The parked thread
// can't see what f wrote before f is done, and the waker is not in the waiter any more once the
// thread has seen it, so the cell and the waiter may be destroyed as soon as gp_wait returns.

#pragma once

#include <cstdint>
#include <type_traits>

namespace gp {

class waiter {
public:
    waiter();
    ~waiter();
    waiter(const waiter&) = delete;
    waiter& operator=(const waiter&) = delete;

    // gp_park blocks until the permit is available, and takes it.
    void gp_park();

    // gp_try_park takes the permit if it is there, without waiting, and says whether it was.
    bool gp_try_park();

    // gp_park_until is gp_park that gives up at the time t, a time point of any clock (the clock
    // is read to see how long that is, so it should be steady_clock: the wait doesn't depend on
    // the wall clock): it returns true when it took the permit, false when t came first (a
    // permit that arrives at t, or before the thread runs again, still counts). It also works for
    // a time very far away.
    template <typename TimePoint>
    bool gp_park_until(TimePoint t) {
        return gp_park_ns(gp_ns(t - TimePoint::clock::now()));
    }

    // gp_park_for is gp_park_until at d from now: d is any std::chrono::duration (a negative or
    // zero one is a poll).
    template <typename Duration>
    bool gp_park_for(Duration d) {
        return gp_park_ns(gp_ns(d));
    }

    // gp_unpark makes the permit available (it is one permit, however often it is called)
    // and wakes the thread that waits for it, if any.
    void gp_unpark();

    // gp_unpark_with(f) runs f under the waiter's lock, then makes the permit available and wakes
    // the parked thread, in one step (see the top). f must not block.
    template <typename F>
    void gp_unpark_with(F&& f) {
        gp_unpark_with_([](void* p) { (*static_cast<std::remove_reference_t<F>*>(p))(); }, &f);
    }

    // gp_wait(pred) blocks until pred() is true. pred runs under the waiter's lock. It takes the
    // permit away when it returns (the wake-up that it waited for set it), so no stale permit is
    // left for the next gp_park of this goroutine.
    template <typename P>
    void gp_wait(P&& pred) {
        gp_wait_([](void* p) -> bool { return (*static_cast<std::remove_reference_t<P>*>(p))(); },
                 &pred, false, 0);
    }

    // gp_wait_until(t, pred) is gp_wait that gives up at the time t: true when pred() is true,
    // false when t came first (pred is false then).
    template <typename TimePoint, typename P>
    bool gp_wait_until(TimePoint t, P&& pred) {
        return gp_wait_(
            [](void* p) -> bool { return (*static_cast<std::remove_reference_t<P>*>(p))(); }, &pred,
            true, gp_ns(t - TimePoint::clock::now()));
    }

    // gp_post(f, ctx) asks the thread that waits on this waiter (in gp_park, gp_wait and the timed
    // forms) to call f(ctx) on its own thread, from inside its wait, and goes on to wait for what
    // it was waiting for. The deadlock report uses it to have every parked goroutine capture its
    // own stack (a thread cannot walk another's portably). It returns false when a request is
    // pending already. The thread that is not in a wait yet serves it when it gets there.
    // gp_post_wait(ns) waits until f has returned (true), or ns nanoseconds went by without the
    // thread taking the request (false: the request is cancelled and ctx is not used any more). One
    // asker at a time.
    bool gp_post(void (*f)(void*), void* ctx);
    bool gp_post_wait(int64_t ns);

private:
    // The nanoseconds of a duration of the C++ library, rounded up, at most about 100 years (a
    // very long wait is the same as for ever, and the arithmetic stays in range).
    template <typename Duration>
    static int64_t gp_ns(Duration d) {
        constexpr double far = 100.0 * 365 * 24 * 3600 * 1e9;
        double ns = static_cast<double>(d.count()) * static_cast<double>(Duration::period::num) *
                    1e9 / static_cast<double>(Duration::period::den);
        if (!(ns < far)) {
            return static_cast<int64_t>(far);
        }
        if (ns <= 0) {
            return ns < -far ? -static_cast<int64_t>(far) : static_cast<int64_t>(ns);
        }
        auto whole = static_cast<int64_t>(ns);
        return static_cast<double>(whole) < ns ? whole + 1 : whole;
    }

    bool gp_park_ns(int64_t ns);
    void gp_unpark_with_(void (*f)(void*), void* ctx);
    // gp_wait_ waits until pred(ctx) is true, for at most ns nanoseconds if timed (ns may be
    // negative: the time has passed), and says whether pred is true.
    bool gp_wait_(bool (*pred)(void*), void* ctx, bool timed, int64_t ns);

    struct gp_state;
    gp_state* gp_s;
};

} // namespace gp
