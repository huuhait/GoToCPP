# ADR-041: Package time is Go's own code, with natives for the clocks and one timer thread

Status: accepted (2026-10-18)

## Context

Step 8.8 builds `time`. Most of it is pure Go (`format.go`, `time.go`, `zoneinfo.go`: Format, Parse,
Date arithmetic, zones, `Duration`), and ADR-018 says the standard library is ported from Go's source.
What is not pure Go is the system and the runtime: the clocks, `Sleep`, the timers behind `Timer`,
`Ticker`, `After`, `AfterFunc`, and the files of the time zone database. Go 1.23 also changed what a
timer channel is: it is unbuffered as far as `len` and `cap` can tell, `Stop` and `Reset` never leave a
stale value in it, and a `Timer` or `Ticker` that nothing references is collected although it was not
stopped. With reference counting there is no collector, so "collected" has to mean "the count goes to
zero", and nothing in a timer queue may keep a `Timer` alive.

## Decision

- **`stdlib/time` is Go 1.25's files, ported by hand** (`PORT.md` lists the differences): `format.gp`,
  `format_rfc3339.gp`, `time.gp`, `zoneinfo.gp`, `zoneinfo_read.gp`, `zoneinfo_unix.gp`. The generic
  helpers of `format.go` (`atoi[bytes []byte | string]` and four more) are written for `string`,
  because generic functions are Phase 07; `math/bits`, `internal/stringslite`, `syscall`, `sync` and
  `unsafe` are replaced by a few lines or by natives. `sleep.gp` and `tick.gp` are Go's with the
  timer calls changed to the natives below.
- **Natives are standard C++** (`runtime/src/time.cpp`, `runtime/src/timer.cpp`, declared in
  `gocpp/time.hpp`, ADR-027): the clocks are `std::chrono::steady_clock` (CLOCK_MONOTONIC, through the
  vDSO) and `system_clock`, the files are C stdio, `getenv` is `std::getenv`, and `call_once` is
  `std::call_once` (package `sync` is step 8.7's, so `time` does not use it). Nothing is in `src/os/`,
  and the same code serves Windows; only the zone sources of `zoneinfo_unix.gp` are per system
  (`zoneinfo_windows.gp` is a stub: UTC, and `ZONEINFO`, until Phase 14).
- **A timer is a record on a channel object.** `chan_core` has a `timer` pointer (set once, before the
  channel is shared). A `Timer` or `Ticker` is `{C, r}` where `r` is that channel (`C` itself for a
  channel timer; a channel nobody uses for `AfterFunc`). The channel has a buffer of one element and
  reports `len == cap == 0`.
- **One timer thread, started when the first timer is armed.** A `std::map` ordered by (deadline,
  arming order) holds the armed timers; the thread waits on a condition variable until the earliest
  deadline, takes the due entry out, and either calls the Go function `sendTime(c, due)` (a send that
  never blocks: `try_send`, so a ticker drops the ticks of a slow receiver) or starts a goroutine that
  runs the `AfterFunc` function. The thread is not a goroutine: not counted, never in the way of the
  program's end (the program leaves with `_exit`). A program that never arms a timer pays nothing.
- **Go 1.23's rules** are Go's algorithm (`runtime/time.go`): `Stop` and `Reset` take the record's
  `fire_mu`, which the thread holds while a send runs, count a send that has not started as stopped
  (`sending`, `seq`), take the timer out of the map, and drain the channel's buffer; a value that was
  there makes the result `true` (the timer was "active": nobody has received its value).
  `Timer.Stop` is `true` exactly until the value is received. The ticker's next deadline is on the
  grid of its period (`when + period*(1 + delay/period)`), as in Go.
- **Weak references without a collector.** The map holds a channel timer's channel by `weak_ptr`; when
  the last reference to the channel goes (the `Timer`, the `Ticker` and the `C` the program kept), the
  record goes with it and takes itself out of the map in its destructor. So `time.After(time.Hour)` in
  a loop, `time.Tick` in a loop and `NewTimer` without `Stop` leave nothing behind. An `AfterFunc`
  timer is held strongly until it fires or is stopped, as in Go (its function may be the only thing
  that will ever run). A closure that holds its own `Timer` is a reference cycle (chan -> record -> fn -> timer -> chan) that
  is never broken, even after the timer has fired (divergence D1), as any cycle is: LeakSanitizer reports it,
  tests need `// lsan: cycle`.
- **`Sleep` does not use the thread.** The goroutine waits on its own `gp::waiter` with a deadline (the
  kernel wakes it), not inside a `block_scope`: a sleeping goroutine is not "blocked" for
  `num_blocked()`, so it is never "asleep" for the deadlock detector (step 8.11 needs no case for it).
- **For 8.11**: `gp::detail::timers_can_wake()` says whether an armed timer could still wake a goroutine:
  an `AfterFunc` timer, or a channel timer that a goroutine is blocked on (Go 1.23's rule: a channel
  timer that nobody waits on does not count). As in Go, a pending `AfterFunc` (even for an hour) keeps
  the deadlock detector quiet until it has fired or been stopped.
- **`select` needs nothing special**: a timer channel is an ordinary channel, and a goroutine that waits
  on it (alone or in a `select` with `time.After`) is parked by the channel code on its receive queue,
  as for any channel. The timer thread wakes it with a `try_send`, which is a normal send.
- **A timer that is due is run by whoever looks at its channel** (Go's `maybeRunChan`). A receive, a
  non-blocking receive, and every poll of a `select` on a timer channel first call `timer_observe`
  (`chan_core::recv`, `try_recv`, `select_run`; a channel without a timer pays one null test): if the
  deadline has passed, the send is done at once on the caller's thread, under the record's `fire_mu`,
  and a send that the timer thread has taken but not finished is waited for (`sending`, `firing`: the
  record's `hint` is `INT64_MIN` until the send is over). So `time.After(0)` followed by a `select`
  with a default always takes the case, and a timer whose deadline a `Sleep` or a busy wait has passed
  is always seen, however late the thread is. `len` and `cap` are 0 whatever the state. `Stop` and
  `Reset` already counted a due timer as active. The send gets the time the timer was *for*:
  `sendTime(c, due)` subtracts, on one clock reading, `due` from `Now()`, so a late thread does not
  make the received times late (Go subtracts a delta measured earlier).

## Alternatives

- **Go's lazy timer channels** (the timer is in the heap only while a goroutine waits on the channel,
  and receives and `len` run it if it is due): exact, but every channel operation of `chan.cpp` and of
  `select` would test for a timer, and `select` (step 8.5, merged) would have to as well. The eager thread has
  the same results for `Stop`, `Reset` and the values, because Go's rules are about what a receive can
  see, and a drain on `Stop` and `Reset` gives the same answers.
- **A timer in the thread that holds a strong reference and removes it when the `Timer` is dropped**:
  impossible without a destructor on the Go struct; the channel is the one object that every holder
  of the timer shares.
- **Timers on `gp::waiter` deadlines in the goroutine that waits** (no thread): fine for `Sleep` (done),
  but a `Ticker` ticks while nobody waits, and `AfterFunc` has no goroutine to wait in.
- **`sync.Once` in `zoneinfo.gp`**: package `sync` is not there yet; `std::call_once` through a native
  does the same for the three places that need it.

## Consequences

Good:
- Format, Parse, `Date`, zones and `Duration` are Go's code: every table in the tests is the output of
  real Go, and 51 run tests compare with it.
- Timers cost one thread, one lock and an ordered map; a million unreferenced timers cost nothing after
  the loop; `Sleep` is a futex wait with a deadline.
- The `Timer` of a program can be 10,000 (tested) without 10,000 threads.

Bad:
- A timer fires about as late as a thread wake-up (50 to 100 microseconds on an idle machine, much more
  when the machine is loaded), not when the deadline passes.
- The thread's callback runs Go code (`sendTime`) off any goroutine: it must stay small (a clock read
  and a non-blocking send).
- `time.Local` and `LoadLocation` need the system's zoneinfo: no embedded copy (`time/tzdata` is not
  provided). `LoadLocation` does not look in `$GOROOT/lib/time/zoneinfo.zip` (no GOROOT).

See: [Phase 08 §2.5](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#25-sync-syncatomic-time-runtime).
