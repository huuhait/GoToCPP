// The deadlock detector (gocpp/goroutine.hpp, "Deadlock detection"; Phase 08 step 8.11, ADR-077):
// Go's checkdead for goroutines that are threads. Standard C++ only (the OS is reached through
// gp::os).
//
// The state is a count of the goroutines that are alive and a count of those that are blocked
// (src/goroutine.cpp, sched.hpp); a goroutine that is about to sleep or has ended asks
// deadlock_check, and only when the two counts are equal does deadlock_look run. It then
//
//   1. gives up when something may still wake a goroutine: a thread that is foreign to the runtime,
//      a panic that is running, a timer that can fire into a waiting goroutine or start one;
//   2. collects the goroutine records (the registry) and checks them one by one: each must be
//   blocked
//      and they must be as many as the live goroutines (a goroutine that is starting is in the
//      count and not in the registry yet, so it is "running");
//   3. has every goroutine capture its own stack: the waiter of each parked goroutine runs a
//      function on the goroutine's thread from inside its wait (waiter::gp_post), because a thread
//      cannot walk another's stack portably;
//   4. prints Go's report and ends the process with code 2.
//
// Nothing here runs unless every goroutine is blocked, which is when a program is stuck or about to
// be: the cost of the detector for the program that works is the two loads in deadlock_check.
#include <gocpp/nofortify.h>
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>
#include <gocpp/trace.hpp>
#include "sched.hpp"

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstddef>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>

namespace gp::detail {

namespace {

constinit std::atomic<deadlock_hook> hook{nullptr};

// One detector at a time: a second goroutine that finds every goroutine blocked while the first
// builds the report leaves it to the first.
constinit std::atomic<bool> looking{false};

// How long the report waits for the parked goroutines to give their stacks, all together.
constexpr int64_t capture_budget_ns = 5LL * 1000000000;

// The same for the report of the other goroutines of a program that is ending with a panic (one
// that cannot answer, because it waits in code that is not the runtime's, must not hold the end).
constexpr int64_t other_budget_ns = 1LL * 1000000000;

// A goroutine's stack, captured on its own thread.
struct stack_slot {
    std::vector<uintptr_t> pcs;
    bool complete = false;
};

void grab_stack(stack_slot& s) noexcept {
    try {
        // The whole stack of this thread, to its end, for Go's "...N frames elided..." lines.
        for (std::size_t size = 1024; size <= (std::size_t(1) << 20) && !s.complete; size *= 8) {
            s.pcs.resize(size);
            int n = os::stack(s.pcs.data(), static_cast<int>(size));
            s.pcs.resize(static_cast<std::size_t>(n < 0 ? 0 : n));
            s.complete = static_cast<std::size_t>(n) < size;
        }
    } catch (...) {
        s.pcs.clear(); // no memory: the goroutine is listed without its stack
        s.complete = false;
    }
}

void capture(void* p) {
    grab_stack(*static_cast<stack_slot*>(p));
}

bool is_blocked(const goroutine* g) noexcept {
    return __atomic_load_n(&g->is_blocked, __ATOMIC_SEQ_CST);
}

// append_goroutine appends Go's block for the goroutine g: a blank line, "goroutine N [why]:", its
// stack (slot; none when null, for a goroutine that is running and cannot be walked) and the line
// "created by". idle is the text in the brackets for a goroutine that has no reason.
void append_goroutine(std::string& out, const goroutine& g, const char* idle,
                      const stack_slot* slot, const exe_image* img) {
    // The fields of the record are read once, here (-race does not watch these reads: race.hpp,
    // ignore_reads; no test kills a mutant without it, now that the edge of wait_for_reports orders
    // the reads before the free, but it is cheap and these are unsynchronized reads of the
    // runtime's own fields). locked is written by the goroutine while it runs: an atomic load; id,
    // parent_id and create_pc are written before the goroutine is published and never again.
    const char* why;
    uint64_t id;
    uint64_t parent_id;
    const void* create_pc;
    bool locked;
    {
        race::ignore_reads ignore;
        why = g.reason();
        id = g.id;
        parent_id = g.parent_id;
        create_pc = g.create_pc;
        locked = __atomic_load_n(&g.locked, __ATOMIC_RELAXED) > 0;
    }
    out += "\ngoroutine ";
    print_uint(out, id);
    out += " [";
    out += why != nullptr ? why : idle;
    if (locked) {
        out += ", locked to thread";
    }
    out += "]:\n";
    if (slot != nullptr) {
        append_trace(out, slot->pcs.data(), slot->pcs.size(), slot->complete, img);
    }
    append_created_by(out, create_pc, parent_id);
}

// finish ends the process with the report, or hands it to the test hook.
void finish(const std::string& text) noexcept {
    if (deadlock_hook h = hook.load(std::memory_order_acquire); h != nullptr) {
        h(text.data(), text.size());
        return;
    }
    claim_death();
    std::fflush(nullptr);
    write_stderr(text);
    os::exit(2);
}

constexpr const char none_text[] =
    "fatal error: no goroutines (main called runtime.Goexit) - deadlock!\n";

} // namespace

void enable_deadlock_detector(bool on) noexcept {
    sched::detector_on.store(on, std::memory_order_release);
}

bool deadlock_detector_enabled() noexcept {
    return sched::detector_on.load(std::memory_order_acquire);
}

deadlock_hook set_deadlock_hook(deadlock_hook h) noexcept {
    return hook.exchange(h, std::memory_order_acq_rel);
}

void note_foreign_entry() noexcept {
    if (!sched::foreign_seen.load(std::memory_order_relaxed)) {
        sched::foreign_seen.store(true, std::memory_order_release);
    }
}

bool foreign_entered() noexcept {
    return sched::foreign_seen.load(std::memory_order_acquire);
}

void deadlock_look() noexcept {
    // (A panic that is running is no reason to stay quiet: its deferred calls that wait for ever
    // are a deadlock, in Go too. Go's `panicking` counts the program that is dying, which here is
    // claim_death: the thread that reports takes it, and the others wait.)
    if (sched::foreign_seen.load(std::memory_order_acquire)) {
        return;
    }
    // Both counts at one instant: if every live goroutine is blocked, nothing runs that could end,
    // wake or start one (but a timer, below), and the records stay as they are while we read them.
    uint64_t v = sched::counts.load(std::memory_order_acquire);
    int64_t live = sched::live_of(v);
    if (live != sched::blocked_of(v)) {
        return;
    }
    if (live == 0) {
        // Main called Goexit and the last goroutine has ended. (An AfterFunc timer that is armed
        // starts one, as in Go, where the program waits for it.)
        if (!sched::main_gone.load(std::memory_order_acquire) || timers_can_wake()) {
            return;
        }
        if (looking.exchange(true)) {
            return;
        }
        finish(none_text);
        looking.store(false);
        return;
    }
    if (timers_can_wake()) {
        return; // a timer may still wake (or start) a goroutine: Go 1.23's rule
    }
    if (looking.exchange(true)) {
        return;
    }
    std::vector<goroutine*> gs;
    bool ok = true;
    try {
        sched::collect(gs);
    } catch (...) {
        ok = false;
    }
    // Every live goroutine is in the registry and blocked (a goroutine that is starting is a live
    // one that is not in it yet). The timers are asked again: one that fired meanwhile may have
    // woken or started a goroutine, and the counts are read again after.
    ok = ok && static_cast<int64_t>(gs.size()) == live;
    for (std::size_t i = 0; ok && i < gs.size(); i++) {
        ok = is_blocked(gs[i]);
    }
    ok = ok && sched::counts.load(std::memory_order_seq_cst) == v && !timers_can_wake();
    if (!ok) {
        looking.store(false);
        return;
    }

    std::string out;
    try {
        std::sort(gs.begin(), gs.end(),
                  [](const goroutine* a, const goroutine* b) { return a->id < b->id; });
        // The stacks: each parked goroutine captures its own, on its own thread; this thread's
        // goroutine (if it is one: it is about to park) captures its own here.
        auto* self = static_cast<goroutine*>(os::tls_get());
        std::vector<stack_slot> slots(gs.size());
        std::vector<char> posted(gs.size(), 0);
        // GOTRACEBACK=none (or 0, or a word that Go does not know): the message only, as Go prints
        // it, so no goroutine is asked for its stack.
        const bool traced = traceback_level() > 0;
        if (!traced) {
            gs.clear();
            slots.clear();
            posted.clear();
        }
        for (std::size_t i = 0; i < gs.size(); i++) {
            if (gs[i] == self) {
                grab_stack(slots[i]);
            } else {
                posted[i] = gs[i]->park.gp_post(&capture, &slots[i]) ? 1 : 0;
            }
        }
        auto end = std::chrono::steady_clock::now() + std::chrono::nanoseconds(capture_budget_ns);
        for (std::size_t i = 0; i < gs.size(); i++) {
            if (posted[i] != 0) {
                auto left = std::chrono::duration_cast<std::chrono::nanoseconds>(
                                end - std::chrono::steady_clock::now())
                                .count();
                gs[i]->park.gp_post_wait(left > 0 ? left : 0);
            }
        }
        out = "fatal error: all goroutines are asleep - deadlock!\n";
        const exe_image* img = program_image();
        for (std::size_t i = 0; i < gs.size(); i++) {
            append_goroutine(out, *gs[i], "waiting", &slots[i], img);
        }
    } catch (...) {
        // Without memory the report is the message.
        out = "fatal error: all goroutines are asleep - deadlock!\n";
    }
    finish(out);
    looking.store(false);
}

void append_other_goroutines(std::string& out) noexcept {
    try {
        // The records stay alive while the scope lives: the other goroutines run, and one that
        // ends waits for the report before it frees its record.
        sched::report_scope scope;
        if (!scope.usable()) {
            // The barrier failed: no record can be read safely (report_scope). Go prints the other
            // goroutines here, so say why they are missing.
            out += "\n(other goroutines not shown: the process barrier failed)\n";
            return;
        }
        std::vector<goroutine*> gs;
        std::vector<char> parked; // 1: blocked in the runtime, or sleeping
        {
            // (-race does not watch these reads of the runtime's records: race.hpp, ignore_reads.)
            race::ignore_reads ignore;
            scope.collect_held(gs);
            auto* self = static_cast<goroutine*>(os::tls_get());
            gs.erase(std::remove(gs.begin(), gs.end(), self), gs.end());
            std::sort(gs.begin(), gs.end(),
                      [](const goroutine* a, const goroutine* b) { return a->id < b->id; });
            for (const goroutine* g : gs) {
                parked.push_back(is_blocked(g) || g->reason() != nullptr ? 1 : 0);
            }
        }

        // (Under -race the request below and the wait for its answer are a mutex and a condition
        // variable that the detector sees: they order this thread with each goroutine that it
        // dumps, which can hide a real race between the two; docs/guides/race.md, Limits.)
        // A goroutine that is blocked in the runtime, or sleeps (it has a reason without a block),
        // captures its own stack on its own thread; one that runs cannot be walked from here, so it
        // is listed without a stack.
        std::vector<stack_slot> slots(gs.size());
        std::vector<char> posted(gs.size(), 0);
        for (std::size_t i = 0; i < gs.size(); i++) {
            if (parked[i] != 0) {
                posted[i] = gs[i]->park.gp_post(&capture, &slots[i]) ? 1 : 0;
            }
        }
        auto end = std::chrono::steady_clock::now() + std::chrono::nanoseconds(other_budget_ns);
        for (std::size_t i = 0; i < gs.size(); i++) {
            if (posted[i] != 0) {
                auto left = std::chrono::duration_cast<std::chrono::nanoseconds>(
                                end - std::chrono::steady_clock::now())
                                .count();
                posted[i] = gs[i]->park.gp_post_wait(left > 0 ? left : 0) ? 1 : 0;
            }
        }
        const exe_image* img = program_image();
        for (std::size_t i = 0; i < gs.size(); i++) {
            append_goroutine(out, *gs[i], "running", posted[i] != 0 ? &slots[i] : nullptr, img);
        }
    } catch (...) {
        // Without memory the report has the goroutines that it has.
    }
}

} // namespace gp::detail
