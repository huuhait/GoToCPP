// The count of the waits that want fine timer ticks (ADR-132) and the slots of the precise waits
// (ADR-133), src/timed_wait.hpp. Standard C++ only: the requests and the slots are the OS layer's.
#include <gocpp/nofortify.h>
#include "timed_wait.hpp"

#include <mutex>
#include <thread>

namespace gp::detail {

namespace {

// The count is under a mutex, not atomic: a request to the OS made when the count leaves 0 must
// not pass the release made when it came back to 0 in another thread. It is never destroyed (a
// wait may end while the process does).
struct fine_state {
    std::mutex mu;
    std::size_t holders = 0;
};

fine_state& fine = *new fine_state;

// The slots of the waits, never destroyed (see slot_pool). They are made only when the system has
// high-resolution timers.
slot_pool<os::wait_slot>& slots = *new slot_pool<os::wait_slot>(
    {&os::wait_slot_new, &os::wait_slot_close, &os::wait_slot_open}, 64);

} // namespace

void fine_timer_acquire() noexcept {
    std::lock_guard<std::mutex> lk(fine.mu);
    if (fine.holders++ == 0) {
        os::fine_timer(true);
    }
}

void fine_timer_release() noexcept {
    std::lock_guard<std::mutex> lk(fine.mu);
    if (--fine.holders == 0) {
        os::fine_timer(false);
    }
}

std::size_t fine_timer_holders() noexcept {
    std::lock_guard<std::mutex> lk(fine.mu);
    return fine.holders;
}

bool os_backend::available() noexcept {
    return os::precise_timers();
}

os::wait_slot* os_backend::acquire() {
    os::wait_slot* s = slots.acquire();
    if (s != nullptr) {
        os::wait_slot_arm(s);
    }
    return s;
}

void os_backend::release(os::wait_slot* s) {
    slots.release(s);
}

std::size_t precise_slots_made() noexcept {
    return slots.made();
}

std::size_t precise_slots_open() noexcept {
    return slots.made() - slots.closed();
}

void precise_sleep(int64_t ns) {
    if (ns <= 0) {
        return;
    }
    if (os_backend::available()) {
        if (auto* s = os_backend::acquire()) {
            os_backend::wait(s, ns);
            os_backend::release(s);
            return;
        }
    }
    auto plan = plan_timed_wait(ns);
    fine_timer_scope fine(plan.fine);
    std::this_thread::sleep_for(plan.wait);
}

} // namespace gp::detail
