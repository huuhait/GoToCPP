# ADR-026: Channels are Go's `hchan` on a mutex, and goroutine threads are reused

Status: accepted (2026-10-18)

## Context

[ADR-007](007-goroutines-os-threads.md) made every goroutine an OS thread, with every blocking
operation going through `gp::waiter`. Steps 8.2 to 8.4 had to decide how channels are built on it,
what a `go` statement lowers to, and what to do about programs that start very many goroutines
(Go programs often start 10k to 1M). Two constraints came from the rest of the runtime: every
generated file includes `gocpp/rt.hpp`, whose compile time is most of the cost of a small program
(`<chrono>` alone is 1.3 s of it and `<condition_variable>` 0.6 s more; `<mutex>` and `<atomic>` are in
it already, through `ptr.hpp`), and the names of its standard headers' macros are reserved for the
emitter (`cxx_macros.txt`).

## Decision

- **A channel is Go's `hchan`**: a `std::mutex`, a ring buffer, and two first-in-first-out queues of
  `sudog`s. A `sudog` is the entry of one waiting goroutine for one operation; it lives on that
  goroutine's stack, and holds its waiter, a pointer to the value, and a pointer to a "done" cell.
  The goroutine that completes the operation copies the value to or from the entry under the
  channel's lock, and wakes the goroutine with `waiter::gp_unpark_with` (which sets the cell under
  the waiter's lock); the sleeper waits with `gp_wait`, which reads the cell under the same lock,
  so the sleeper cannot go on, end, and free its record before the waker is out of it. The entry
  of a `select` case (step 8.5) is the same record, with a claim flag that the cases share.
  A waiting goroutine spins for about 4 µs before it sleeps.
- **`gp::chan<T>` is a `std::shared_ptr` to the channel object** (16 bytes). `chan T`, `<-chan T`
  and `chan<- T` are one C++ type; the direction is in the type descriptor only. The channel object
  is not a template: it works on element pointers through a table of the element type's size, move
  and zero functions, so the code exists once. Its source is not in a public header
  (`src/chan_core.hpp`).
- **`go f(args)` is lowered by the code of `defer f(args)`** (one function, `deferredCall`, with a
  switch that says "start a goroutine" instead of "push on the frame"): `gp::go_fn` for a function
  of the program, `gp::go_call` for the rest.
- **Threads are reused.** When a goroutine ends, its thread waits for the next `go` statement (up to
  256 idle threads, 5 seconds each, only for the default stack size) instead of exiting. Starting a
  thread costs 15 to 60 µs, reusing one about 4 µs. Goroutine ids and records are new for each
  goroutine. A sanitizer build keeps every thread (LeakSanitizer cannot stop a thread that is
  exiting).
- **The headers stay light**: `waiter.hpp`, `goroutine.hpp` and `chan.hpp` are in `rt.hpp` and
  include no `<condition_variable>` and no `<chrono>` (and `<mutex>` and `<atomic>` only
  through what `rt.hpp` has anyway). The waiter is a pointer to
  its state (`src/waiter.cpp`), its timed calls are templates over any duration or time point of the
  C++ library, and the atomics in the headers are the compilers' builtins.
- **A panic in a goroutine is not lost when main returns**: `run_main` waits (at most 200 ms) for
  goroutines that are running the deferred calls of a panic, and for a goroutine that has claimed
  the program's death, before it ends the program with code 0 (Go's `runningPanicDefers`).
- **A thread's alternate signal stack is `malloc`ed**, not a local array of the thread's start
  function: a frame of 64 KiB is probed page by page (`-fstack-clash-protection`), so each thread
  touched 64 KiB. A blocked goroutine now costs about 14 KB of resident memory instead of 73 KB.

## Alternatives

- **`std::atomic::wait`/`notify` (a futex) instead of the mutex and the condition variable**:
  faster for a hand-off, but a waiter then needs a deadline-aware wait (timers, step 8.8) which the
  standard does not have before C++26. The waiter is behind one small class, so it can change.
- **Per-goroutine stacks smaller than 8 MiB**: measured to save nothing on 64-bit Linux (untouched
  pages are free) and a 256 KiB stack overflowed at a recursion depth of 2,000.
- **Putting the concurrency headers in a header of their own that the emitter includes only when a
  program uses them**: every such program pays more than a second for `<chrono>`, and the macro
  list grows for all of them.
- **A fixed pool of worker threads that runs goroutines to completion**: a blocked goroutine would
  hold its worker, so a program with more blocked goroutines than workers would deadlock. Reusing
  only the threads of *finished* goroutines never changes what a program can do.

## Consequences

Good:
- A channel is the same algorithm as Go's, with Go's panics, FIFO wake-up and happens-before edges,
  and ThreadSanitizer sees the mutex.
- A loop of a million short goroutines takes about 6 µs for each and 3 MB of memory.
- `rt.hpp` costs the same to compile as before.

Bad:
- A hand-off between two goroutines is a system call and a context switch when the receiver sleeps
  (7 to 15 µs on a loaded machine, against 200 ns in Go). The spin helps when the other goroutine
  is running: about 1 µs for each operation of an unbuffered ping-pong on idle cores.
- The number of goroutines that live at the same time is bounded by the system's thread limits
  (measured: 100,000 work on a machine with generous limits; see D7 in
  [the language spec](../spec/language.md#d7-goroutines-are-os-threads)).

See: [Phase 08 §2.3](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#23-channels-gos-hchan-algorithm).
