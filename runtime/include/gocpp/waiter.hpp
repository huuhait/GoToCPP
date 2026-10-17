// gocpp/waiter.hpp: gp::waiter, how a goroutine blocks.
//
// Every blocking operation of the runtime (a channel send or receive, select, sync's Mutex,
// WaitGroup and Cond, time.Sleep) parks the goroutine's thread on a waiter, and whoever makes
// the operation possible unparks it (ADR-007). It is Go's gopark and goready, with the OS
// thread as the goroutine: a mutex and a condition variable of the C++ library (so it builds
// on every OS the runtime does; the Windows port needs nothing here).
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
#pragma once

#include <chrono>
#include <condition_variable>
#include <mutex>
#include <utility>

namespace gp {

class waiter {
public:
    waiter() = default;
    waiter(const waiter&) = delete;
    waiter& operator=(const waiter&) = delete;

    // gp_park blocks until the permit is available, and takes it.
    void gp_park() {
        std::unique_lock<std::mutex> lk(gp_mu);
        gp_cv.wait(lk, [this] { return gp_permit; });
        gp_permit = false;
    }

    // gp_try_park takes the permit if it is there, without waiting, and says whether it was.
    bool gp_try_park() {
        std::lock_guard<std::mutex> lk(gp_mu);
        return std::exchange(gp_permit, false);
    }

    // gp_park_until is gp_park that gives up at the time t: it returns true when it took the
    // permit, false when t came first (a permit that arrives at t, or before the thread runs
    // again, still counts). It also works for a time very far away, and doesn't depend on the
    // wall clock.
    bool gp_park_until(std::chrono::steady_clock::time_point t) {
        std::unique_lock<std::mutex> lk(gp_mu);
        // Waiting in steps of at most a day keeps the C++ library's conversion of a deadline
        // from overflowing (a time like time_point::max()).
        constexpr auto step = std::chrono::hours(24);
        while (!gp_permit) {
            auto left = t - std::chrono::steady_clock::now();
            if (left <= std::chrono::steady_clock::duration::zero()) {
                return false;
            }
            gp_cv.wait_for(lk, left < step ? left : step);
        }
        gp_permit = false;
        return true;
    }

    // gp_park_for is gp_park_until at d from now.
    template <typename Rep, typename Period>
    bool gp_park_for(std::chrono::duration<Rep, Period> d) {
        using clock = std::chrono::steady_clock;
        constexpr auto far = std::chrono::hours(24 * 365 * 100);
        if (d >= far) {
            return gp_park_until(clock::now() + far);
        }
        return gp_park_until(clock::now() + std::chrono::ceil<clock::duration>(d));
    }

    // gp_unpark makes the permit available (it is one permit, however often it is called)
    // and wakes the thread that waits for it, if any.
    void gp_unpark() {
        // Under the lock: see the top of the file.
        std::lock_guard<std::mutex> lk(gp_mu);
        gp_permit = true;
        gp_cv.notify_one();
    }

private:
    std::mutex gp_mu;
    std::condition_variable gp_cv;
    bool gp_permit = false;
};

} // namespace gp
