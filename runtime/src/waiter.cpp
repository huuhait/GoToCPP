// gp::waiter (gocpp/waiter.hpp): a mutex, a condition variable and a permit.
#include <gocpp/nofortify.h>
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
    std::condition_variable cv;
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

// wait_step waits on cv for the time that is left, or one step of it (see timed_wait.hpp: the
// last part of a long wait asks the system for accurate timers, the rest does not).
void wait_step(std::unique_lock<std::mutex>& lk, std::condition_variable& cv, clk::duration left) {
    auto next = left < step ? left : step;
    auto plan =
        detail::plan_timed_wait(std::chrono::duration_cast<std::chrono::nanoseconds>(next).count());
    detail::fine_timer_scope fine(plan.fine);
    cv.wait_for(lk, plan.wait);
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
        gp_s->cv.wait(lk);
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
            gp_s->cv.wait(lk);
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
