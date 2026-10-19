# ADR-077: The deadlock detector reads one word of counts, and a registry of worker threads

Status: accepted (2026-10-19)

## Context

Step 8.11. Go ends a program in which every goroutine is asleep with `fatal error: all goroutines are
asleep - deadlock!`, a list of every goroutine (its reason, its stack, where it was created) and exit
code 2 (`checkdead` in `runtime/proc.go`). It reports `fatal error: no goroutines (main called
runtime.Goexit) - deadlock!` when `main` called `runtime.Goexit` and no goroutine is left. Go 1.23 counts
a timer as something that can wake a goroutine only when it can still fire into a waiting goroutine or
start one; a program that uses cgo never reports a deadlock, because a thread of C code may call back.

gocpp had the parts since 8.3: `block_scope` (a goroutine that parks says why and is counted by
`num_blocked()`), `unblock` by the waker before the wake-up, `timers_can_wake()`. What was missing: the
decision, the list of goroutines (a goroutine is a thread: the process has no table of them), the
stacks of the other threads, and the cases (Goexit of main, foreign threads, in-flight timers).

## Decision

- **One word of counts.** `live` (goroutines alive) and `blocked` (goroutines inside a `block_scope`)
  are the two halves of one `atomic<uint64_t>` (`sched::counts`). A goroutine that is about to sleep
  (in `park_cell`, after its entry is queued and the locks are released) and a goroutine that has ended
  call `deadlock_check()`: one load, and the slow path only when the halves are equal. A goroutine that
  falls asleep reads the shared word only if its own block made every live goroutine blocked
  (`block_enter` keeps the value its `fetch_add` returned and sets `last_asleep`; the equality can newly
  arise only at a `block_enter`, a goroutine end, `main_goexit` or a timer firing, and each of those
  checks): the first version read the word in every park, which cost 7 to 11% on independent
  ping-pong pairs at 16 to 32 threads (cache-line traffic). Read at one
  instant, "every live goroutine is blocked" means nothing runs that could wake, end or start one, so
  the state is stable and the records can be read without a fence on the goroutine side. (Two
  separate counters leave a window in which the check sees one of them before and one after a change,
  and then the detector has to protect the records it reads against goroutines that end meanwhile: a
  hand-shake that costs every goroutine end a sequentially consistent store, which is a full fence
  on x86, and that the first version had.)
- **The registry is the workers.** A worker (a goroutine thread, `goroutine.cpp`) publishes the record
  of the goroutine it runs in an atomic pointer on a cache line of its own (a plain store at the start
  and at the end of a goroutine); the workers are a list under a mutex that only starting and ending
  a thread takes. The main goroutine's record is one more pointer. The detector walks the list, and
  requires as many records as live goroutines, all with the blocked flag (a goroutine that is starting
  is live but not in the list: it is running).
- **Stacks come from the goroutines themselves.** A thread cannot walk another's stack portably (no
  signals, no `SuspendThread`). The detector posts a function on the `waiter` of every parked goroutine
  (`gp_post`); the goroutine runs it on its own thread from inside its wait loop (`os::stack`), and the
  detector waits for all of them (`gp_post_wait`, five seconds in all). The same code serves Linux and
  Windows. The waiter's wait loops test one more flag.
- **"Created by"**: `go_start` records the creator's id and the return address of `gp::go`, `go_fn` or
  `go_call` (which are `noinline` for that: their return address is in the function with the `go`
  statement and its line is the statement's). Two stores per `go` statement.
- **The report** costs about 0.06 ms of CPU per goroutine besides the thread's own wake-up (the stack
  walk, the names of the functions are resolved once per process, not once per goroutine; 3000
  goroutines in a debug build: 0.3 s in all, most of it starting the threads).
- **The report** is Go's: `fatal error: all goroutines are asleep - deadlock!`, then for each goroutine
  in the order of its id a blank line, `goroutine N [reason]:`, the frames, and `created by F in
  goroutine P` with its file and line. One thread builds it (`claim_death`), writes it and ends the
  process with code 2.
- **What may still wake a goroutine** (the detector says nothing then): a goroutine in `Sleep`, in a
  system call or running (they are not blocked); `timers_can_wake()` (an armed `AfterFunc`, a channel
  timer that a goroutine waits on, and now a timer the thread has taken out of the heap and not
  finished firing: `heap::busy`); and a **foreign
  thread**: a thread that no `go` statement started and that asked for its goroutine record (a callback
  from a C library's thread, a `std::thread` that used a channel), or a callback converted to a C
  function pointer, turns the detector off for good (`sched::foreign_seen`), as Go does with cgo. The
  timer thread is the runtime's own (`adopt_internal`). `gp::lib` never turns the detector on
  (`run_main` does).
- **A panic is not a reason to stay quiet.** Go's `panicking` counts the program that is dying, not a
  panic whose deferred calls run; a deferred call that waits for ever (also after `recover()`) is a
  deadlock there, and here. (The first version of the detector had a guard on the number of panics being
  handled; it hid that case.) The thread that is ending the program (`claim_death`) is what keeps the
  others quiet.
- **`locked to thread`.** `runtime.LockOSThread` is still nothing, but the goroutine counts the calls
  and the report adds Go's `, locked to thread` after the reason; the main goroutine is locked while the
  package initializers run (a deadlock in `init` says so). A goroutine that an `AfterFunc` timer
  started is "created by time.goFunc" (Go prints the position of `time/sleep.go:215`, which is copied).
- **`main` called `runtime.Goexit`.** `run_main` calls `main_goexit()`: the main goroutine is not
  counted any more, out of the registry, and its thread waits. The goroutine that ends last finds `live
  == 0` with `main_gone` and prints the "no goroutines" message (no trace); when the goroutines that are
  left are all asleep it is the ordinary report, without goroutine 1. An armed `AfterFunc` keeps the
  program (it will start a goroutine), as in Go.
- **Tests** set a hook (`set_deadlock_hook`) that gets the report instead of the end of the process.

## Alternatives

- **A polling thread** that looks every few milliseconds: costs a thread and CPU on every program,
  reports late, and sees torn states; the event-driven check costs a load where a goroutine sleeps.
- **A registry of all goroutine records under a lock** (or sharded): every `go` statement and every
  goroutine end would take a lock; the workers are the threads that exist anyway.
- **Signals (`tgkill` and a handler) to get other threads' stacks**: Linux only, needs
  async-signal-safe unwinding, and a different mechanism for Windows.
- **Reporting only goroutine 1** (what `fatal` prints for other fatal errors): Go lists every goroutine
  in this report, and a deadlock is mostly diagnosed by the other goroutines' reasons.

## Consequences

Good:
- `<-ch` with no sender, `select {}`, a send to a full channel, a Mutex locked twice, a WaitGroup that
  never reaches zero, a Cond nobody signals, a range over a channel nobody closes: all end with Go's
  message and exit code 2 instead of hanging; the tests compare their stderr with real Go's.
- The cost for programs that work is a load where a goroutine sleeps or ends, two stores per `go`
  statement, a store at the start and the end of each goroutine, and a call that `go_call` is not
  inlined for (its return address). The counts were two words that each goroutine start, end, sleep
  and wake already updated; they are one word now.

Bad:
- A goroutine that waits in a way that is not a `block_scope` (a wait for a child process, a blocking
  read) is "running": a program stuck there is not reported (Go reports a goroutine in a system call
  as running too).
- A thread of C code that has not called into gocpp yet is not known: a program in which `main` waits
  for a callback that C will make later is reported as a deadlock when the callback was not converted
  to a function pointer in the program (Go with cgo never reports one).
- The report has no `, N minutes` after the reason of a goroutine that waited a long time (a clock
  read for every block would cost the hot path), and the frames of `sync` are gocpp's
  (`sync.(*Mutex).Lock`), not Go's runtime frames (`internal/sync.runtime_SemacquireMutex`).
- `runtime.Stack(buf, true)` still shows only the calling goroutine.
- `-release` builds are stripped: the frames are `?()` with addresses.

See: [Phase 08 §2.6](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#26-program-exit-and-deadlocks).
