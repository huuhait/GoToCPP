# Phase 08: Concurrency (goroutines, channels, select, sync)

> Make Go-style concurrent code work: `go`, channels, `select`, `sync`, `sync/atomic` and
> timers. Each `go` statement starts a **real OS thread**: simple, robust, easy to debug in gdb.
> There is no goroutine scheduler, now or later (your decision, 2026-10-13). The names stay Go's
> (`go`, "goroutine", `runtime.NumGoroutine`), so Go code and Go's standard library port as is.

| | |
|---|---|
| Depends on | Phase 05 (panics, `Goexit` signal, interfaces), Phase 07 (generic APIs: `atomic.Pointer[T]`, `sync.OnceValue`) |
| Unlocks | Phase 09 (`context`, `time`, `os/exec`, …), Phase 14 (the thread layer gets its Windows version) |
| Milestone | **M5: Go-style concurrency works** |
| Size | large (about 4–5 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Goroutines and channels are a big part of why people enjoy Go. The semantics are well
defined, and Go's runtime source shows exactly how channels and `select` work (the `hchan`,
`sudog` and `selectgo` algorithms in `runtime/chan.go` and `runtime/select.go`). gocpp
re-implements those algorithms in C++20.

The one design rule that matters most: **every blocking operation goes through a small
`gp::waiter` (park/unpark) interface**, a condition variable on the OS thread. Channels,
`select`, `sync` and timers are written against it, and thread creation goes through one small
OS layer (`gp::os`, which lives in the runtime's `runtime/src/os/` folder like every other OS
call), so the Windows port (Phase 14) touches only these two places.

## 2. How it works

### 2.1 Example

```go
func worker(jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs {
		results <- j * j
	}
}

func main() {
	jobs, results := make(chan int, 100), make(chan int, 100)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go worker(jobs, results, &wg)
	}
	for i := 1; i <= 9; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)

	select {
	case r := <-results:
		fmt.Println("first:", r)
	case <-time.After(time.Second):
		fmt.Println("timeout")
	}
}
```

```cpp
// `go f(args)`: Go evaluates f and the arguments NOW, in the calling goroutine
gp::go([jobs, results, wg] { worker(jobs, results, wg); });   // wg is boxed: &wg was taken

gp::send(jobs, i);                    // blocks while the buffer is full
gp::close(jobs);

{   // select (step 8.5: one object for each case, one call, a switch on the number of the case)
    gp::sel_recv gp_s0{results};                          // case 0: r := <-results
    gp::sel_recv gp_s1{::gpkg_time::After(::gpkg_time::Second), gp::sel_drop};   // case 1 (the case holds the timer channel)
    switch (gp::select(gp_s0, gp_s1)) {                   // random choice among ready cases, like Go
    case 0: { int64_t r = gp_s0.gp_take(); ::gpkg_fmt::Println({"first:"_gs, r}); } break;
    case 1: ::gpkg_fmt::Println({"timeout"_gs}); break;
    }
}
```

### 2.2 Goroutines = OS threads

- `gp::go(fn)` starts a detached POSIX thread (on Windows, Phase 14, the same API through
  MinGW-w64's winpthreads, linked with `-pthread`). Its stack size is configurable through
  `pthread_attr_setstacksize` (`std::thread` can't set it). The default is **8 MiB**, settable
  down to 1 MiB. Small stacks save nothing on 64-bit Linux, because stack pages only cost memory
  once they are touched. The audit measured the same RSS for 10k threads at 64 KiB, 256 KiB and
  8 MiB, while a 256 KiB stack overflowed on a modest recursion (depth 2000).
- **Stack overflow is reported Go-style.** A guard page plus a `SIGSEGV` handler running on an
  alternate signal stack (`sigaltstack`) prints `fatal error: stack overflow` and exits with
  code 2, instead of a bare segfault.
- It gets a goroutine id for panic messages and is counted for `runtime.NumGoroutine()`.
- Each goroutine body runs inside a `try`. An unrecovered panic prints
  `panic: … goroutine N [running]:` and **exits the whole program with code 2**, as in Go.
- Practical limit: **tens of thousands** of goroutines (the audit ran 50k threads at about
  410 MiB RSS; the kernel limits on this machine allow far more). Programs that need millions
  of goroutines are not a target.
- **Threads are reused (step 8.2, a deviation from "each `go` statement starts a thread").** When a
  goroutine ends, its thread waits for the next `go` statement (up to 256 idle threads, 5 seconds
  each, default stack size only) instead of exiting. Starting a thread costs 15 to 60 µs; handing a
  goroutine to an idle one costs about 4 µs, and a loop of a million short goroutines runs on a few
  hundred threads. Ids are not reused. Measured on this machine (Linux 6.19, `max_map_count` 1M,
  a shared machine under load): 1,000 / 10,000 / 100,000 goroutines blocked on a channel start in
  0.1 s / 0.8 s / 13 s and take 7 MB / 65 MB / 1.3 GB (about 14 KB each: the thread's alternate
  signal stack is `malloc`ed, because a 64 KiB local array in the thread's start function was
  probed page by page by `-fstack-clash-protection` and cost 64 KiB each). Systems with the
  default `vm.max_map_count` of 65,530 and no `MADV_GUARD_INSTALL` (before Linux 6.13) stop at 20,000
  to 30,000 threads. What would help beyond that is not done: smaller stacks do not save memory
  (untouched pages are free) and overflow early, and a thread pool that runs goroutines to
  completion deadlocks when more goroutines block than there are workers.
- `runtime.LockOSThread` is a no-op that always holds: every goroutine already has its own
  thread, which is exactly what C libraries such as OpenGL or GUI toolkits need.

### 2.3 Channels (Go's `hchan` algorithm)

`gp::chan<T>` is a shared reference to `{mutex, ring buffer, recvq, sendq, closed}`:

| Operation | Behavior |
|---|---|
| send | closed → panic `send on closed channel`. A receiver is waiting → hand the value over directly and wake it. Buffer has room → enqueue. Otherwise park in `sendq` |
| receive | A sender is waiting → take its value (unbuffered: directly; buffered: from the buffer head, and move the sender's value to the tail). Buffer not empty → dequeue. Closed → zero value, `ok = false`. Otherwise park in `recvq` |
| close | nil → panic `close of nil channel`; already closed → panic `close of closed channel`; else wake all receivers (zero, false) and all senders (they panic) |
| nil channel | send/receive block forever; in `select` a nil case is never ready |
| `len`, `cap`, `for v := range ch` | as in Go |

Implementation notes (steps 8.3 and 8.4): the channel object is not a template (it works on
element pointers with a table of the element type's size, move and zero functions) and lives in
`runtime/src/chan.cpp` and the internal `src/chan_core.hpp`; `sudog` is a record on the waiting
goroutine's stack with the waiter, a pointer to the value and a pointer to a "done" cell, plus the
claim flag and case index that `select` will use (one entry per case, all sharing one cell and one
claim flag; `waitq::dequeue` already skips an entry whose claim is taken). A waker finishes the
operation under the channel's lock, unlocks, and tells the sleeper with `waiter::gp_unpark_with`; a
sleeper that wakes late cannot free its record while the waker is still in it. A waiting goroutine
spins for about 4 µs first (a ping-pong between two running goroutines then needs no system call).
Blocked goroutines are counted (`gp::num_blocked()`, `block_scope`, the reason in the goroutine's
record) for the deadlock detector: a goroutine that a waker has handed a value is not counted any
more from the moment the waker wakes it (`unblock`), so `num_blocked() == num_goroutine()` is not true
while a wake-up is on its way; timers, `sync` and a goroutine that is starting are still 8.11's to
account for. **Steps 8.5 (`select`) and 8.8 (`time`, `sync`) must park through `block_scope` entered
under the lock before the entry is queued, and wake through `wake()`/`unblock`**, so that the
exactly-once decrement of the blocked count still holds (the mutant "block_exit never decrements"
is equivalent only while every wake goes through `wake()`). A panic in a goroutine is waited for
when main returns (`settle_panics`; the count follows the `panic_error` object, tests can cut the
wait with `set_settle_timeout`). Because `rt.hpp` includes these headers, `<mutex>`,
`<condition_variable>`, `<chrono>` and `<atomic>` are not in them: the waiter is a pointer to its
state, defined in `src/waiter.cpp` (ADR-026).

Directional types (`<-chan T`, `chan<- T`) are the same `gp::chan<T>` in C++. They are different
Go types, so they get different type descriptors (Phase 05): `%T` prints `<-chan int`, and a
type switch can tell them apart.

### 2.4 `select` (Go's `selectgo` algorithm)

1. Collect the cases, shuffle the **poll order** randomly, and sort the **lock order** by
   channel address (deduplicated), so two selects can never deadlock each other.
2. Lock all channels. If any case is ready (in poll order), do it, unlock, done.
3. If there is a `default`, unlock and run it.
4. Otherwise enqueue **one waiter entry per case** (like Go's `sudog`: each has its own value slot
   and case index), all sharing **one claim flag**, then unlock and park.
5. The goroutine that completes one case first **claims** the select with an atomic
   compare-and-swap on that flag (Go's `selectDone`), so two channels can never both complete the
   same select. After waking up, lock all channels again, remove the other entries, and run the
   winning case. This also works when the same channel appears in two cases.

Implementation notes (step 8.5, [ADR-029](../../../docs/adr/029-select-and-range-over-channels.md)):
`runtime/src/select.cpp` is this algorithm on the channel objects of 8.3; the poll of one channel under
its lock (`send_locked`, `recv_locked`) is the code of the plain operations too. A case is an object
(`gp::sel_recv`, `gp::sel_send`, `gp::sel_default`, [select.hpp](../../../runtime/include/gocpp/select.hpp)) and a
select is `switch (gp::select(cases...))`, so `break`, `continue`, `return` and `goto` in a case are plain
C++ (a lambda for each case would break them). Evaluation order is the order of the declarations; the left-hand
sides of `case a[i], ok = <-c:` are evaluated only if the case is chosen. A send on a closed channel panics when the poll
reaches it (a ready receive earlier in the poll order wins, as in Go). Up to 16 cases allocate nothing.
`select {}` and a select of nil channels only block for ever and count as blocked (`select (no
cases)`, `select`). Differences from the plan text: the poll order comes from a thread-local xorshift64*, not
Go's `cheaprand`; the type checker's messages are go/types'
(`select case must be send or receive (possibly with assignment)`, `multiple defaults (first at ...)`).
`for v := range ch` (step 8.6) is `gp::range_chan` / `gp::range_chan_drop` with `gp::range_next`. For step 8.8:
`case <-time.After(d)` needs nothing new (the case holds the timer's channel, a channel like any other, and the
timer thread wakes the select through the channel's `try_send` and `wake()`); for step 8.11: a goroutine in a
select is in `block_scope("select")` from before its entries are visible until a waker has unblocked it, so
`num_blocked()` is exact for it. ThreadSanitizer's deadlock detector aborts above 64 locks held by a thread, so
step 8.12's job must set `TSAN_OPTIONS=detect_deadlocks=0` for the `select_1000_cases` test.

### 2.5 `sync`, `sync/atomic`, `time`, `runtime`

- **`sync.Mutex` is not `std::mutex`.** Go allows one goroutine to lock and *another* to unlock,
  which is undefined behavior for `std::mutex`. So `Mutex`/`RWMutex` are a small atomic state
  word, and waiters wait **outside** the object (on the address, through `std::atomic::wait` or a
  futex-style table keyed by address, as Go's `runtime/sema.go` does). That keeps a `Mutex`
  copyable like in Go (copying copies the state word; `go vet` warns, and so will `gocpp vet`)
  without duplicating queue pointers.
- `WaitGroup` (including Go 1.25's `wg.Go(f)`), `Once` (counts as done even if `f` panics, as in
  Go), `OnceFunc`/`OnceValue`/`OnceValues` (generic), `Cond`, `Map`, `Pool` (simple versions).
- `sync/atomic`: the typed API (`atomic.Int64`, `atomic.Bool`, `atomic.Value`, and
  `atomic.Pointer[T]` on a runtime `gp::atomic_ptr<T>`, a small lock around a `gp::ptr`, because
  libc++ (used by llvm-mingw on Windows) doesn't implement `std::atomic<std::shared_ptr>` and libstdc++'s isn't lock-free
  anyway), plus the function-style API (`atomic.AddInt64(&x, 1)` → `std::atomic_ref`).
- **`time`, with Go 1.23+ timer rules:** `Sleep`, and a runtime **timer thread** (a min-heap of
  deadlines) that drives `After`, `Tick`, `NewTimer`, `NewTicker`, `AfterFunc`, `Stop`, `Reset`.
  Since Go 1.23, timer channels are unbuffered (`cap(t.C) == 0`), `Stop`/`Reset` guarantee no
  stale value arrives afterwards, and a Timer or Ticker nobody references is collected even if it
  was never stopped, so `time.Tick` in a loop no longer leaks. With reference counting, that means
  the timer heap holds **weak** references.
- `runtime`: `Gosched`, `NumGoroutine`, `GOMAXPROCS` (the default follows the CPU count and,
  like Go 1.25, cgroup limits and CPU affinity; setting it stores the value, so
  `GOMAXPROCS(1)` then `GOMAXPROCS(0)` returns 1, even though gocpp doesn't use it for scheduling),
  `Goexit` (the Phase 05 `goexit_signal`: defers run, `recover()` returns nil, the goroutine ends).
- `iter.Pull` (Phase 07) can be built on a helper thread.

### 2.6 Program exit and deadlocks

- **One exit path, always the same:** when `main` returns, on `os.Exit(n)` or on a fatal error,
  flush stdout/stderr, then call `_exit`. C++ static destructors do **not** run at exit. Neither
  Go nor gocpp runs anything at exit, and running destructors while goroutine threads are still
  alive would be a race. `_exit` (not `_Exit`) is used because sanitizers intercept it: with
  `_Exit`, ThreadSanitizer printed a race but the program still exited 0 (found in the audit). The
  test harness also fails any run whose output contains `WARNING: ThreadSanitizer`.
- **Deadlock detection (best effort, following Go):**
  - the runtime counts goroutines blocked in gocpp operations (channels, `select`, `WaitGroup`,
    `Mutex`, `Cond`). If all goroutines are blocked and nothing can wake them, it prints
    `fatal error: all goroutines are asleep - deadlock!` and exits with code 2;
  - "nothing can wake them" follows Go 1.23+: an `AfterFunc`, a `Sleep` or a channel timer that
    someone is waiting on counts as a possible wake-up, but a channel timer nobody waits on does
    not;
  - a C/C++ thread that ever entered gocpp code (a callback from a C library) counts as
    running, because it may wake `main` (Go handles this case correctly too);
  - if `main` calls `runtime.Goexit` and no goroutines are left, the message is Go's:
    `fatal error: no goroutines (main called runtime.Goexit) - deadlock!`.

## 3. Steps

1. **8.1 `gp::waiter`** (park/unpark) + goroutine runtime (spawn with stack size, ids, counts,
   panic → exit 2, stack-overflow handler).
2. **8.2 `go` statement codegen** (function value and arguments evaluated first, closures,
   method values).
3. **8.3 Channels** (unbuffered + buffered, send/recv/recv-ok/close/len/cap, nil channels, panics).
4. **8.4 Directional channel types** (same C++ type, distinct descriptors).
5. **8.5 `select`** (random poll order, address lock order, default, nil cases, one entry per
   case + shared claim flag).
6. **8.6 `for range ch`.**
7. **8.7 `sync`** (Mutex/RWMutex on atomics with waiters outside the object, WaitGroup + `Go`,
   Once and the generic `OnceFunc`/`OnceValue`, Cond, Map, Pool) + `sync/atomic` (typed API,
   `gp::atomic_ptr<T>`, function-style API via `std::atomic_ref`).
8. **8.8 `time`** with Go 1.23+ timer rules (unbuffered timer channels, no stale values,
   weak references in the timer heap).
9. **8.9 Exit semantics** (flush + `_exit`, no static destructors, TSan-aware harness). Done: `gp::exit`, ADR-038; the harness fails a run with a ThreadSanitizer report; the ASan leak check runs before `_exit`.
10. **8.10 `runtime`** (Gosched, NumGoroutine, GOMAXPROCS that remembers its value, Goexit). The first of these makes the `runtime` package exist: the gate rows for `runtime` and `runtime/cgo` are at Phase 09 since step 5.12 (`internal/check/gates.go`), so 8.10 must move the `runtime` row to Phase 08 (and add `runtime.Error`/`PanicNilError`, which it owns unless 9.2 does). Done (ADR-038): the package is `stdlib/runtime` (+ `runtime/debug`), the row is at Phase 08, `runtime/cgo` stays at 09.
11. **8.11 Deadlock detection** (Go 1.23+ timer rule, foreign threads, the `Goexit` message).
12. **8.12 `-race` CI job** (ThreadSanitizer over the concurrency suite).
13. **8.13 Lift the gates; tests**: ≥ 50 concurrency run tests (deterministic output only); the
    **self-contained** programs from Go's `test/chan` (`fifo`, `goroutines`, `nonblock`, `select`,
    `select3`, `select4`, `select6`, `select7`, `select8`, `sendstmt`, `sieve1`, `zerosize`,
    `powser1`, `powser2`) as differential tests, taken from the pinned upstream Go source
    (NixOS's GOROOT has no `test/` folder); stress tests (10k goroutines, 1M channel operations);
    a deep-recursion test; timer `cap`/`len`/`Stop`/`Reset` differential tests. The other
    `test/chan` programs need `flag`, `runtime.GC`, `container/*` or `text/template` and move to
    Phase 09; `perm.go` is an error test (`tests/errors`).
14. **8.14 (optional) `iter.Pull` on a helper thread.**
15. **8.15 Thread layer for portability**: thread start with a stack size, thread-local storage
    and the stack-overflow hook behind one small `gp::os` interface in `runtime/src/os/`, with the Linux version here
    and the Windows version in Phase 14.

## 4. Deliverables

- Runtime: goroutines, channels, select, sync, atomic, timers, deadlock detector
- Codegen for `go`, channel operations, `select`, `range` over channels
- Target program `workers` passing

## 5. Exit criteria

- `workers` example passes on both compilers.
- The concurrency suite is green under ThreadSanitizer on both compilers, with the harness failing
  on any TSan warning.
- 10k simultaneous goroutines run without failure; a deep recursion prints
  `fatal error: stack overflow` instead of a bare segfault.
- The deadlock tests print Go's exact messages (incl. the `Goexit` case) and exit with code 2.
- The self-contained `test/chan` programs produce the same output as Go.

## 6. Risks

| Risk | Mitigation |
|---|---|
| OS threads are heavy | Measured: tens of thousands are fine; millions of goroutines are not a target (decision) |
| Subtle channel/select bugs | Port Go's algorithm faithfully; Go's `test/chan` as differential tests; TSan |
| Exit behavior hides sanitizer findings | One flush + `_exit` path; harness fails on TSan warnings |
| Mutex unlocked by another goroutine | Atomic/waiter-based `Mutex`, never `std::mutex` |
| ARC refcounts under contention | Atomic already; measure in Phase 15 |
