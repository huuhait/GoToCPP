# ADR-007: Goroutines are OS threads, with no scheduler

Status: accepted (2026-10-13)

## Context

Goroutines and channels are a big part of Go. Go's runtime runs many goroutines on a few
threads with its own scheduler, which is large and complex. The user decided that `go` starts a
real OS thread and that there is no goroutine scheduler, now or later
([Q4](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

## Decision

- `go f(x)` evaluates `f` and its arguments in the calling goroutine (Go's rule), then starts a
  detached OS thread.
- No goroutine scheduler, now or later. The names stay Go's (`go`, goroutine,
  `runtime.NumGoroutine`), so Go code and Go's standard library port as they are.
- The default stack is 8 MiB and can be set down to 1 MiB. A stack overflow prints
  `fatal error: stack overflow` and exits with code 2.
- Every blocking operation goes through one small `gp::waiter` (park/unpark) interface.
  Channels and `select` copy Go's own algorithms (`hchan`, `selectgo`). `sync.Mutex` is an atomic
  state word, not `std::mutex`, because Go lets another goroutine unlock it.
- An unrecovered panic in any goroutine ends the program with code 2, as in Go.
  `runtime.LockOSThread` is a no-op that always holds.
- Threads start through the thread layer `gp::os` in `runtime/src/os/`
  ([ADR-021](021-platforms-linux-windows.md)); on Windows that is winpthreads, linked with
  `-pthread`.

```go
for range 4 {
	wg.Add(1)
	go worker(jobs, results, &wg) // each goroutine is a real OS thread
}
```

```cpp
gp::go([jobs, results, wg] { worker(jobs, results, wg); });   // wg is boxed: &wg was taken
```

## Alternatives

- **Fibers or an M:N scheduler** (rejected): complex and not needed; the fiber "v2" was dropped.
- **No `go` at all** (rejected): not Go-like.

## Consequences

Good:
- Simple, robust and easy to debug: in gdb every goroutine is a normal thread.
- C libraries that need one fixed thread (OpenGL, GUI toolkits) just work.

Bad:
- Tens of thousands of goroutines are fine, millions are not (D7). Programs that need millions
  are not a target.
- `runtime.GOMAXPROCS` stores and returns its value, but nothing schedules by it; `iter.Pull`
  needs a helper thread.
- Deadlock detection is best effort: it counts goroutines blocked in gocpp operations.

## Evidence

- Audit: 50k threads ran at about 410 MiB RSS. 10k threads used the same RSS with 64 KiB,
  256 KiB and 8 MiB stacks (stack pages cost memory only once touched), while a 256 KiB stack
  overflowed at recursion depth 2000. That is why the default is 8 MiB.

See: [Phase 08 §1](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#1-idea), [§2.2](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#22-goroutines--os-threads), [§2.5](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#25-sync-syncatomic-time-runtime), [§2.6](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#26-program-exit-and-deadlocks), [Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen), divergence [D7](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
