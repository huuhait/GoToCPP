// gp::waiter (gocpp/waiter.hpp): a mutex, a condition variable and a permit.
#include <gocpp/nofortify.h>
#include <gocpp/gc/hooks.hpp>
#include <gocpp/waiter.hpp>
#include "timed_wait.hpp"

#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <mutex>

namespace gp {

// A request of gp_post: allocated by the asker (the deadlock report is the only one), so the state
// of every goroutine's waiter stays small.
struct post_ext {
    void (*fn)(void*) = nullptr;
    void* ctx = nullptr;
    bool done = false;
    std::condition_variable done_cv;
};

struct waiter::gp_state {
    std::mutex mu;
    detail::precise_cv cv;
    bool permit = false;
    // A request that the parked thread runs a function of the asker's (gp_post), for the deadlock
    // report: req is set under mu by the asker and cleared by the parked thread when it takes it;
    // ext has the function and says when it has returned.
    bool req = false;
    post_ext* ext = nullptr;

    ~gp_state() { delete ext; }

    // serve runs the posted function on this thread (lk is held; it is released while the function
    // runs, so a waker is not held up), and tells the asker.
    void serve(std::unique_lock<std::mutex>& lk) {
        post_ext* e = ext;
        req = false;
        lk.unlock();
        e->fn(e->ctx);
        lk.lock();
        e->done = true;
        e->done_cv.notify_all();
    }
};

namespace {

using clk = std::chrono::steady_clock;

// Waiting in steps of at most a day keeps the C++ library's conversion of a deadline from
// overflowing.
constexpr auto step = std::chrono::hours(24);

// wait_call is the data of one blocking wait: the condition variable, the lock that the wait
// releases while it waits, and the timeout (timed is false for an untimed wait).
struct wait_call {
    std::unique_lock<std::mutex>* lk;
    detail::precise_cv* cv;
    std::chrono::nanoseconds ns;
    bool timed;
};

// wait_now is the callback of the blocking section: it only waits (phase-16 README §2.5: the
// frames inside the section are not scanned, so it must not allocate and must not take a runtime
// lock). The waiter's own mutex is held when the section begins and is released by the wait
// itself, so a waker is never held up by the section; the lock is not a runtime lock that the
// collector could hold.
void wait_now(void* p) {
    auto* c = static_cast<wait_call*>(p);
    if (c->timed) {
        c->cv->wait_for(*c->lk, c->ns);
    } else {
        c->cv->wait(*c->lk);
    }
}

// gc_wait and gc_wait_for are the waiter's two waits, inside blocking sections when a GC engine
// is linked (gocpp/gc/hooks.hpp): a parked goroutine is not stopped by the collector and its
// stack is scanned from the point of the section, not through it. The callback only waits, so
// pred() and the cells that a waker hands over stay in scanned frames.
void gc_wait(std::unique_lock<std::mutex>& lk, detail::precise_cv& cv) {
    wait_call call{&lk, &cv, std::chrono::nanoseconds(0), false};
    detail::gc_blocking_call(wait_now, &call);
}

void gc_wait_for(std::unique_lock<std::mutex>& lk, detail::precise_cv& cv,
                 std::chrono::nanoseconds ns) {
    wait_call call{&lk, &cv, ns, true};
    detail::gc_blocking_call(wait_now, &call);
}

// wait_step waits on cv for the time that is left, or one step of it (see timed_wait.hpp: where
// the system has precise timers the wait is on one, and elsewhere the last part of a long wait
// asks for finer system ticks).
void wait_step(std::unique_lock<std::mutex>& lk, detail::precise_cv& cv, clk::duration left) {
    auto next = left < step ? left : step;
    gc_wait_for(lk, cv, std::chrono::duration_cast<std::chrono::nanoseconds>(next));
}

} // namespace

waiter::waiter() : gp_s(new gp_state) {}

waiter::~waiter() {
    delete gp_s;
}

void waiter::gp_park() {
    std::unique_lock<std::mutex> lk(gp_s->mu);
    while (!gp_s->permit) {
        if (gp_s->req) {
            gp_s->serve(lk);
            continue;
        }
        gc_wait(lk, gp_s->cv);
    }
    gp_s->permit = false;
}

bool waiter::gp_try_park() {
    std::lock_guard<std::mutex> lk(gp_s->mu);
    bool had = gp_s->permit;
    gp_s->permit = false;
    return had;
}

bool waiter::gp_park_ns(int64_t ns) {
    std::unique_lock<std::mutex> lk(gp_s->mu);
    auto deadline = clk::now() + std::chrono::nanoseconds(ns);
    while (!gp_s->permit) {
        if (gp_s->req) {
            gp_s->serve(lk);
            continue;
        }
        auto left = deadline - clk::now();
        if (left <= clk::duration::zero()) {
            return false;
        }
        wait_step(lk, gp_s->cv, left);
    }
    gp_s->permit = false;
    return true;
}

void waiter::gp_unpark() {
    // Under the lock: see gocpp/waiter.hpp.
    std::lock_guard<std::mutex> lk(gp_s->mu);
    gp_s->permit = true;
    gp_s->cv.notify_one();
}

void waiter::gp_unpark_with_(void (*f)(void*), void* ctx) {
    std::lock_guard<std::mutex> lk(gp_s->mu);
    f(ctx);
    gp_s->permit = true;
    gp_s->cv.notify_one();
}

bool waiter::gp_wait_(bool (*pred)(void*), void* ctx, bool timed, int64_t ns) {
    std::unique_lock<std::mutex> lk(gp_s->mu);
    if (!timed) {
        while (!pred(ctx)) {
            if (gp_s->req) {
                gp_s->serve(lk);
                continue;
            }
            gc_wait(lk, gp_s->cv);
        }
    } else {
        auto deadline = clk::now() + std::chrono::nanoseconds(ns);
        while (!pred(ctx)) {
            if (gp_s->req) {
                gp_s->serve(lk);
                continue;
            }
            auto left = deadline - clk::now();
            if (left <= clk::duration::zero()) {
                return false;
            }
            wait_step(lk, gp_s->cv, left);
        }
    }
    gp_s->permit = false;
    return true;
}

bool waiter::gp_post(void (*f)(void*), void* ctx) {
    std::lock_guard<std::mutex> lk(gp_s->mu);
    if (gp_s->req) {
        return false; // one request at a time
    }
    if (gp_s->ext == nullptr) {
        gp_s->ext = new post_ext;
    }
    gp_s->ext->fn = f;
    gp_s->ext->ctx = ctx;
    gp_s->ext->done = false;
    gp_s->req = true;
    gp_s->cv.notify_one();
    return true;
}

// gp_post_wait is the asker's side of gp_post: it waits for the parked thread to take the
// request, and then for the function to return (the deadlock report's stack capture). Its two
// waits are NOT blocking sections yet: the asker is a running goroutine, not a parked one, and
// 16.4 wraps the runtime's remaining waits (this one, the timer thread's own wait, and
// wait_for_reports' lock wait).
bool waiter::gp_post_wait(int64_t ns) {
    std::unique_lock<std::mutex> lk(gp_s->mu);
    post_ext* e = gp_s->ext;
    e->done_cv.wait_for(lk, std::chrono::nanoseconds(ns), [e] { return e->done; });
    if (!e->done && gp_s->req) {
        gp_s->req = false; // the thread never took it: cancelled
        return false;
    }
    // Taken, and maybe still running: the function's context must stay valid until it is done.
    e->done_cv.wait(lk, [e] { return e->done; });
    return true;
}

} // namespace gp
