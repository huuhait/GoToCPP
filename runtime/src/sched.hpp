// The runtime's scheduler state that more than one of its files reads (internal to the runtime:
// src/goroutine.cpp owns it, src/deadlock.cpp reads it for the deadlock detector, step 8.11,
// ADR-077). Nothing here is part of the runtime's interface for generated code.
#pragma once

#include <gocpp/goroutine.hpp>
#include <gocpp/race.hpp>

#include <atomic>
#include <cstdint>
#include <vector>

namespace gp::detail::sched {

// The goroutines that are alive (num_goroutine) in the high half and the goroutines that are
// blocked in a gocpp operation (num_blocked, block_scope) in the low half of one word: the detector
// reads both at one instant (every live goroutine is blocked: nothing runs that could end, wake or
// start one, so the goroutine records stay as they are), and no per-goroutine fence is needed.
extern std::atomic<uint64_t> counts;
inline constexpr uint64_t live_one = uint64_t(1) << 32;
inline constexpr int64_t live_of(uint64_t v) noexcept {
    return static_cast<int64_t>(v >> 32);
}
inline constexpr int64_t blocked_of(uint64_t v) noexcept {
    return static_cast<int64_t>(v & 0xffffffffu);
}

// The detector is off until run_main turns it on (a program: not a shared library that a host
// program loads, not a unit test that blocks on purpose).
extern std::atomic<bool> detector_on;
// Some thread that no go statement started has entered gocpp (a C library's thread that called a
// callback, a std::thread that sent on a channel), or a callback was handed out as a C function
// pointer: such a thread may wake any goroutine at any time, so nothing is ever "asleep" (what Go
// does in a program that uses cgo, where it never reports a deadlock).
extern std::atomic<bool> foreign_seen;
// The main goroutine called runtime.Goexit: it is not counted any more and not in the registry.
extern std::atomic<bool> main_gone;
// What a goroutine that ends reads for the report of all goroutines (see goroutine.cpp), kept on a
// cache line of its own: the lines around the counts are written by every goroutine start and
// end, and these three words are almost never written, so a read of them must not share a line
// with anything that is.
//   barrier_end:    the system has a process-wide barrier (os::process_barrier_init succeeded,
//                   run_main set it before any goroutine exists): a goroutine that ends pays no
//                   fence for the report, the report makes the barrier instead.
//   reporting:      how many reports exist now (the goroutine that ends looks at it).
//   barrier_failed: the barrier failed once; no report reads a record any more.
struct alignas(64) report_flags {
    std::atomic<bool> barrier_end{false};
    std::atomic<int> reporting{0};
    std::atomic<bool> barrier_failed{false};
};
// (The line is the whole object: aligned to 64 and padded to 64, so nothing else is on it.)
static_assert(alignof(report_flags) == 64 && sizeof(report_flags) == 64,
              "report_flags must own a cache line");
extern report_flags report_state;
inline std::atomic<bool>& barrier_end = report_state.barrier_end;

// The goroutines of the registry: the main goroutine's record and the record that each goroutine
// thread (worker) is running now. collect appends them (the caller sorts them by id); it is for the
// detector, which calls it when every live goroutine is blocked (see counts).
//
// The registry costs the goroutine hot paths a plain store each: a worker publishes the record that
// it runs and withdraws it at the end. The list of workers has a lock that only starting and
// ending a thread takes, and the detector.
void collect(std::vector<goroutine*>& out);

// reports_in_progress is how many report scopes exist now (a goroutine that ends looks at whether
// it is not 0); forget_barrier_failure is for the tests: it makes the scopes usable again.
int reports_in_progress() noexcept;
void forget_barrier_failure() noexcept;

// A report of all goroutines while the others run (the traceback of a panic with GOTRACEBACK=all,
// append_other_goroutines): the records that collect_held returns stay alive while the scope lives.
// A goroutine that ends takes the lock of the registry before it frees its record whenever a scope
// exists (reporting is not 0), so it waits for the report; the report holds the lock while it
// reads. Not for the deadlock detector, which needs no scope: nothing runs then.
//
// Where the system has a process-wide barrier the scope makes it (os::process_barrier). If the
// system call fails (after process_barrier_init had succeeded: a kernel or sandbox that took the
// command away), the goroutines that already ended with the cheap store cannot be ordered against
// this report, so no record may be read: usable() is false, for this scope and for every later one
// (a goroutine that read barrier_end before the failure may still be on its way out), and the
// report lists no other goroutine. The goroutines go on as before.
struct report_scope {
    report_scope();
    ~report_scope();
    report_scope(const report_scope&) = delete;
    report_scope& operator=(const report_scope&) = delete;
    bool usable() const noexcept;
    void collect_held(std::vector<goroutine*>& out);
};

} // namespace gp::detail::sched
