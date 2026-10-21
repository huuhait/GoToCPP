// The accuracy of the runtime's timed waits (internal to the runtime: src/timed_wait.cpp; the
// decision is ADR-132). A wait with a timeout, on a condition variable or in sleep_for, can end
// late by a tick of the system timer: nothing on Linux, up to 15.6 ms on Windows, unless the
// process has asked for finer ticks (os::fine_timer). Asking costs power (the whole machine ticks
// faster while any process does), so a wait asks only for the end of its time, not for all of it:
//
//     auto plan = plan_timed_wait(left_ns);
//     fine_timer_scope fine(plan.fine);
//     cv.wait_for(lk, plan.wait);
//
// plan_timed_wait splits a wait of left_ns into a coarse first part and a fine last part, and the
// caller waits again (its loop) when the first part ends. Where os::timer_margin_ns is 0 (Linux),
// the plan is the whole wait, with no fine ticks: the code runs as if this file were not there.
#pragma once

#include <gocpp/os.hpp>

#include <chrono>
#include <cstddef>
#include <cstdint>

namespace gp::detail {

// One step of a timed wait: wait this long, with fine ticks if fine.
struct timed_step {
    std::chrono::nanoseconds wait;
    bool fine;
};

// plan_timed_wait is the next step of a wait that has left_ns (positive) to go, for the rounding
// margin of the system: a wait of more than two margins first waits for all but one margin, with
// ordinary ticks (they add at most one margin, so the wait ends inside the last margin), and a
// wait of at most two margins is a fine step to the end. A margin of 0 is the whole wait, coarse.
constexpr timed_step plan_timed_wait(int64_t left_ns, int64_t margin_ns) noexcept {
    if (margin_ns <= 0) {
        return {std::chrono::nanoseconds(left_ns), false};
    }
    if (left_ns > 2 * margin_ns) {
        return {std::chrono::nanoseconds(left_ns - margin_ns), false};
    }
    return {std::chrono::nanoseconds(left_ns), true};
}

inline timed_step plan_timed_wait(int64_t left_ns) noexcept {
    return plan_timed_wait(left_ns, os::timer_margin_ns());
}

// fine_timer_acquire and fine_timer_release count the waits that want fine ticks, and ask the OS
// (os::fine_timer) when the count goes from 0 to 1 and from 1 to 0, so any number of waits in any
// threads share one request. fine_timer_holders is the count, for tests.
void fine_timer_acquire() noexcept;
void fine_timer_release() noexcept;
std::size_t fine_timer_holders() noexcept;

// A fine_timer_scope holds the fine ticks for the life of the object, if on.
class fine_timer_scope {
public:
    explicit fine_timer_scope(bool on) noexcept : on_(on) {
        if (on_) {
            fine_timer_acquire();
        }
    }
    ~fine_timer_scope() {
        if (on_) {
            fine_timer_release();
        }
    }
    fine_timer_scope(const fine_timer_scope&) = delete;
    fine_timer_scope& operator=(const fine_timer_scope&) = delete;

private:
    bool on_;
};

} // namespace gp::detail
