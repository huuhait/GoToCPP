# ADR-029: `select` is Go's `selectgo` on the channel queues, lowered to case objects and a switch

Status: accepted (2026-10-18)

## Context

Steps 8.3 and 8.4 built the channel (ADR-026): a mutex, a ring buffer, two FIFO queues of `sudog`s,
and entries that already have the claim flag and the case index that `select` needs. Step 8.5 had to
decide how a Go `select` statement is written in C++ (readable, no heap allocation for the usual
small selects, Go's evaluation order), how the algorithm of `runtime/select.go` maps to the channel
object, and what a `for v := range ch` loop is (step 8.6).

## Decision

- **A select statement is a block of one object per case, one call, and a C++ `switch` on the number
  of the case it returns** (`gocpp/select.hpp`):

  ```cpp
  { gp::sel_recv gp_s0{a};  gp::sel_send gp_s1{b, f()};  gp::sel_recv gp_s2{c, gp::sel_drop};
    switch (gp::select(gp_s0, gp_s1, gp_s2, gp::sel_default)) {
    case 0: { int64_t v = gp_s0.gp_take(); ... } break;
    ...
    default: ... } }
  ```

  The objects are declared in source order, so the channel expressions and the values to send are
  evaluated once, in source order, before anything communicates, also for the cases that are not
  chosen. Each case is declared (its channel read, its value copied) before the calls of the next
  case run, and in a send case the channel is read before the value's calls: gc copies each channel
  and value into a temporary in source order (`walk/order.go`, `OSELECT`), so a later call that
  changes a variable changes neither the channel nor the value of an earlier case
  (`select_chan_copy_semantics`; for a plain send statement `c <- f()` gc reads `c` after `f()`). The received value lives in a slot of its own case object (`elem_slot<T>`: no default
  constructor needed, nothing constructed for a case that is not chosen, a move-only or C++ element
  works), and the case's body starts by declaring the case's variables from it (`gp_take()`,
  `gp_ok()`), or assigns them (`case a[i], ok = <-c:` evaluates its left-hand sides only when it is
  chosen). `break` leaves the C++ `switch` as Go's `break` leaves the select, `continue` and the
  labelled jumps go through it to the loop. The number of a case is its position in the list; the
  default is `gp::sel_default` at its position.
- **The algorithm is Go's `selectgo`** (`src/select.cpp`): a random poll order (the insertion shuffle
  of Go, a thread-local xorshift64* seeded from the clock, a counter and the generator's address), a
  lock order sorted by the address of the channel object (a channel in two cases is locked once),
  pass 1 under all the locks (the first ready case in poll order; `default` when none), and pass 2:
  one `sudog` for each case on its channel's queue (the entries share one claim flag and one wake
  cell), `block_scope("select")` entered under the locks before the first entry is visible, unlock,
  park. Whoever completes a case takes the claim flag in `waitq::dequeue` (which drops the entries
  of a select that is already taken), does the transfer under its channel's lock and wakes the
  select with `wake()`; the select locks everything again and removes its other entries. The poll of
  one channel (`send_locked`, `recv_locked`) is the code of the plain `send` and `recv` too, so the
  two cannot differ. Up to 16 cases take no allocation (the order arrays and the entries are on the
  stack; the entries are constructed only when the goroutine blocks).
- **Nil channels are never ready, `select {}` and all-nil selects block for ever and count as blocked**
  (`select (no cases)` and `select` in the traceback and in the blocked count of step 8.11).
  A send on a closed channel panics when the poll reaches it (Go: the same, so a ready receive that
  comes first in the poll order wins), a receive from a closed channel is ready (zero value, `ok`
  false).
- **A channel that is a local variable is held by reference, any other channel expression by a copy of
  the handle.** Nothing can change a local between the evaluation of the cases and the select, and
  the copy of a `shared_ptr` costs two atomic operations and makes the reference count of a channel
  that many goroutines select on a hot cache line. A call's result is held as it is.
- **`for v := range ch` is `for (auto gp_it0 = gp::range_chan(ch); gp::range_next(gp_it0);) { T v =
  gp_it0.gp_v(); ...}`**: the iterator holds the channel (evaluated once) and a slot, so the variable of
  each iteration is a new variable (Go 1.22: closures and `&v` see their own), `for range ch`
  is `gp::range_chan_drop` (receive without a place for the value), a nil channel blocks, and a closed
  channel ends the loop without constructing a zero value (`chan_recv_range`: the element type may have
  none). The code of the loop's variables is the one of the range loops over slices and maps and of a
  select's receive case (`bindVars`).
- **A select without `default` that is a terminating statement in Go** (every case body ends in a
  terminating statement of the spec: return, goto, panic, block, if/else, `for {}` without break,
  switch with default, nested select, labeled statement; no break to the select): the lowering writes
  `gp::select_unreachable();` after its `switch` so that a function can end with it without
  `-Wreturn-type` (a warning also stops the object cache from keeping the compile).

## Alternatives

- **One variadic call with the values as arguments and a lambda per case**: the bodies then are
  lambdas, and `break`, `continue`, `return` and `goto` in a case do not work in C++. The `switch` makes
  them plain C++.
- **A heap-allocated select descriptor** (a vector of cases): costs an allocation in the loops that
  Go programs write around `select`. The case objects live on the stack and the array of type-erased
  entries is on the stack for up to 16 cases.
- **`std::tuple` of the cases and `std::apply`**: unreadable output, and the objects cannot be named
  by the case bodies.
- **A random poll order from `std::random_device` or `rand()`**: a system call or a lock per select.
- **Choosing by "first ready in source order"**: not Go's semantics (starvation); the tests would pass
  by accident on a loaded machine and fail on a quiet one.

## Consequences

Good:
- The two selects of a rendezvous (one sends, one receives, both blocked in a select on several
  channels) can complete each other without a deadlock, and a goroutine woken by one of its channels
  cannot be woken by another (the claim flag): both are tested under ThreadSanitizer with 8 threads.
- The generated code reads like the Go: `gp::select(...)` and a `switch`.
- 8.8 and 8.11 need nothing from select: a timer channel is a channel (`wake()` from the timer's
  `try_send`), and a goroutine in a select is counted by `num_blocked()` exactly once.

Bad:
- A select holds the handle of its non-local channels, the channel of a local variable by
  reference, and the values of its send cases that were not chosen (a large slice sent in an unchosen
  case stays alive) until the end of its C++ block, also while a long case body runs (Go drops them
  at once).
- **ThreadSanitizer's deadlock detector aborts when one thread holds more than 64 locks**, and a
  select locks all its distinct channels at the same time, so a select over more than about 60
  different channels cannot run under TSan unless `TSAN_OPTIONS=detect_deadlocks=0` is set (the
  `select_1000_cases` run test; step 8.12's TSan job must set it). Go's own `selectgo` does the same.
- A select costs about one lock/unlock pair for each distinct channel on every poll, as in Go; the
  measured cost is in the runtime README.

See: [Phase 08 §2.4](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#24-select-gos-selectgo-algorithm),
[ADR-026](026-channels-and-thread-reuse.md).
