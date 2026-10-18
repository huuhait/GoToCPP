// gp::waiter (gocpp/waiter.hpp): a mutex, a condition variable and a permit.
#include <gocpp/nofortify.h>
#include <gocpp/waiter.hpp>

#include <chrono>
#include <condition_variable>
#include <cstdint>
#include <mutex>

namespace gp {

struct waiter::gp_state {
    std::mutex mu;
    std::condition_variable cv;
    bool permit = false;
};

namespace {

using clk = std::chrono::steady_clock;

// Waiting in steps of at most a day keeps the C++ library's conversion of a deadline from
// overflowing.
constexpr auto step = std::chrono::hours(24);

} // namespace

waiter::waiter() : gp_s(new gp_state) {}

waiter::~waiter() {
    delete gp_s;
}

void waiter::gp_park() {
    std::unique_lock<std::mutex> lk(gp_s->mu);
    gp_s->cv.wait(lk, [this] { return gp_s->permit; });
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
        auto left = deadline - clk::now();
        if (left <= clk::duration::zero()) {
            return false;
        }
        gp_s->cv.wait_for(lk, left < step ? left : step);
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
        gp_s->cv.wait(lk, [pred, ctx] { return pred(ctx); });
    } else {
        auto deadline = clk::now() + std::chrono::nanoseconds(ns);
        while (!pred(ctx)) {
            auto left = deadline - clk::now();
            if (left <= clk::duration::zero()) {
                return false;
            }
            gp_s->cv.wait_for(lk, left < step ? left : step);
        }
    }
    gp_s->permit = false;
    return true;
}

} // namespace gp
