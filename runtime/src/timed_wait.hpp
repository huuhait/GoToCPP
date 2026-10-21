// The accuracy of the runtime's timed waits (internal to the runtime: src/timed_wait.cpp; the
// decisions are ADR-132 and ADR-133). A wait with a timeout, on a condition variable or in
// sleep_for, can end late by a tick of the system timer: nothing on Linux, up to 15.6 ms on
// Windows. Two mechanisms, chosen at run time:
//
//   - Where the OS has high-resolution timers (os::precise_timers: Windows 10 1803 and later), a
//     precise_cv waits on a wait_slot of the OS layer, a timer that another thread can wake. The
//     process asks for nothing, the machine keeps its idle tick, and the wait ends within about
//     half a millisecond (ADR-133).
//   - Elsewhere a wait asks for finer system ticks (os::fine_timer) for the end of its time only,
//     not for all of it (the machine ticks faster while any process asks; ADR-132):
//
//         auto plan = plan_timed_wait(left_ns);
//         fine_timer_scope fine(plan.fine);
//         cv.wait_for(lk, plan.wait);
//
//     plan_timed_wait splits a wait of left_ns into a coarse first part and a fine last part, and
//     the caller waits again (its loop) when the first part ends. Where os::timer_margin_ns is 0
//     (Linux), the plan is the whole wait, with no fine ticks: the code runs as if this file were
//     not there.
#pragma once

#include <gocpp/os.hpp>

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstddef>
#include <cstdint>
#include <mutex>
#include <vector>

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

// --- precise waits (ADR-133) ---

// How long before its end a precise wait sets its timer to fire. The timer fires up to about half
// a millisecond after its time (the interrupt that serves it), so the wait sets it to fire this
// early and spins from there to the end: the wait is then on time most of the time, for a few tens
// of microseconds of CPU on average (ADR-133 has the numbers).
inline constexpr int64_t precise_preroll_ns = 300'000;

// How a precise wait of ns nanoseconds is spent: wait on the high-resolution timer for timer_ns,
// then spin for up to spin_ns more. The spin is the pre-roll, but at most half of the wait: a
// wait of 100 us is a timer of 50 us and a spin of at most 50 us (the timer is mostly later than
// that, so the spin is mostly nothing), not a spin of the whole wait, which would use a whole
// core for a loop of short sleeps.
struct precise_split {
    int64_t timer_ns;
    int64_t spin_ns;
};

constexpr precise_split split_precise_wait(int64_t ns, int64_t preroll_ns) noexcept {
    if (ns <= 0) {
        return {0, 0};
    }
    int64_t spin = preroll_ns < ns / 2 ? preroll_ns : ns / 2;
    return {ns - spin, spin};
}

// The slots of the waits that are going on, kept for reuse. A slot is the OS layer's (a timer and
// a way to wake it) and its struct is never destroyed: the thread that wakes a wait may hold the
// pointer a moment after the wait is over, and the wake-up of a slot that has been reused is only
// a spurious wake-up (every wait is in a loop that looks at its condition and the clock). At most
// max_idle idle slots keep their system handles; the others give them back (close) and get them
// again when they are used (open), so a burst of 10000 sleeping goroutines does not leave 20000
// handles behind. made() is how many slots were ever made, the most timed waits that were going
// on at one time.
template <typename Slot>
class slot_pool {
public:
    struct ops {
        Slot* (*make)() noexcept;      // a new slot, open, or null
        void (*close)(Slot*) noexcept; // give the handles of an idle slot back
        bool (*open)(Slot*) noexcept;  // get them again; false if the system cannot
    };

    slot_pool(ops o, std::size_t max_idle) noexcept : ops_(o), max_idle_(max_idle) {}

    // acquire returns an open idle slot, or a new one; null when none can be had.
    Slot* acquire() {
        std::lock_guard<std::mutex> lk(mu_);
        if (!idle_.empty()) {
            Slot* s = idle_.back();
            idle_.pop_back();
            return s;
        }
        if (!closed_.empty()) {
            Slot* s = closed_.back();
            if (!ops_.open(s)) {
                return nullptr;
            }
            closed_.pop_back();
            return s;
        }
        Slot* s = ops_.make();
        if (s != nullptr) {
            made_++;
        }
        return s;
    }

    void release(Slot* s) {
        std::lock_guard<std::mutex> lk(mu_);
        if (idle_.size() < max_idle_) {
            idle_.push_back(s);
            return;
        }
        ops_.close(s);
        closed_.push_back(s);
    }

    // made is how many slots were ever made, idle how many are free and open now, closed how
    // many are free and without handles (for tests).
    std::size_t made() {
        std::lock_guard<std::mutex> lk(mu_);
        return made_;
    }
    std::size_t idle() {
        std::lock_guard<std::mutex> lk(mu_);
        return idle_.size();
    }
    std::size_t closed() {
        std::lock_guard<std::mutex> lk(mu_);
        return closed_.size();
    }

private:
    ops ops_;
    std::size_t max_idle_;
    std::mutex mu_;
    std::vector<Slot*> idle_;
    std::vector<Slot*> closed_;
    std::size_t made_ = 0;
};

// The backend of a precise_cv is the OS layer's slots (os_backend) or, in a test, a fake one:
//   using slot = ...;
//   static bool available();            // precise waits can be used
//   static slot* acquire();             // a slot armed for a wait, or null
//   static void release(slot*);
//   static void wait(slot*, int64_t ns); // blocks for ns or until woken
//   static void wake(slot*);             // ends the wait of the slot, from any thread
struct os_backend {
    using slot = os::wait_slot;
    static bool available() noexcept;
    static slot* acquire();
    static void release(slot* s);
    static void wait(slot* s, int64_t ns) noexcept {
        precise_split split = split_precise_wait(ns, precise_preroll_ns);
        os::wait_slot_wait(s, split.timer_ns, split.spin_ns);
    }
    static void wake(slot* s) noexcept { os::wait_slot_wake(s); }
};

// A condition variable whose timed wait is precise where the system allows (wait_for), and is
// the C++ library's wait with the fine ticks of ADR-132 where it does not. The rest is the
// condition variable's: notify_one, notify_all and wait. At most one thread should wait with a
// timeout at a time (the waits of the runtime are one waiter's); a second one waits the old way.
//
// A thread in a precise wait is not on the condition variable but on a slot, so a notify also
// wakes the slot. The thread puts its slot where a notifier can see it while it holds the mutex,
// and the notifier changed what the thread waits for under the same mutex (or the thread saw the
// change), so the wake-up cannot be lost: the slot keeps a wake-up that comes before the wait.
template <typename Backend>
class basic_precise_cv {
public:
    void notify_one() noexcept {
        cv_.notify_one();
        wake_slot();
    }

    void notify_all() noexcept {
        cv_.notify_all();
        wake_slot();
    }

    void wait(std::unique_lock<std::mutex>& lk) { cv_.wait(lk); }

    // wait_for waits at most ns nanoseconds for a notify, with lk held, and may return earlier
    // (the caller's loop looks at its condition and the clock).
    void wait_for(std::unique_lock<std::mutex>& lk, std::chrono::nanoseconds ns) {
        if (ns.count() <= 0) {
            return;
        }
        if (Backend::available() && slot_.load() == nullptr) {
            if (auto* s = Backend::acquire()) {
                slot_.store(s);
                lk.unlock();
                Backend::wait(s, ns.count());
                lk.lock();
                slot_.store(nullptr);
                Backend::release(s);
                return;
            }
        }
        auto plan = plan_timed_wait(ns.count());
        fine_timer_scope fine(plan.fine);
        cv_.wait_for(lk, plan.wait);
    }

private:
    void wake_slot() noexcept {
        if (auto* s = slot_.load()) {
            Backend::wake(s);
        }
    }

    std::condition_variable cv_;
    std::atomic<typename Backend::slot*> slot_{nullptr};
};

using precise_cv = basic_precise_cv<os_backend>;

// precise_sleep blocks the calling thread for ns nanoseconds, precisely where the system allows
// (ADR-133) and with the fine ticks of ADR-132 otherwise. Nothing wakes it early; it may end a
// little early on the old path, so the caller's loop looks at the clock.
void precise_sleep(int64_t ns);

// precise_slots_made is how many slots of the OS layer the runtime made so far, and
// precise_slots_open how many of them have their system handles now (for tests).
std::size_t precise_slots_made() noexcept;
std::size_t precise_slots_open() noexcept;

} // namespace gp::detail
