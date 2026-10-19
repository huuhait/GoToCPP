// The runtime's scheduler state that more than one of its files reads (internal to the runtime:
// src/goroutine.cpp owns it, src/deadlock.cpp reads it for the deadlock detector, step 8.11,
// ADR-077). Nothing here is part of the runtime's interface for generated code.
#pragma once

#include <gocpp/goroutine.hpp>

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

// The goroutines of the registry: the main goroutine's record and the record that each goroutine
// thread (worker) is running now. collect appends them (the caller sorts them by id); it is for the
// detector, which calls it when every live goroutine is blocked (see counts).
//
// The registry costs the goroutine hot paths a plain store each: a worker publishes the record that
// it runs and withdraws it at the end. The list of workers has a lock that only starting and
// ending a thread takes, and the detector.
void collect(std::vector<goroutine*>& out);

} // namespace gp::detail::sched
