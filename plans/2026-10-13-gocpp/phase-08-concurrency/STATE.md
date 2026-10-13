# Phase 08: Concurrency · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [ ] 8.1 `gp::waiter` (park/unpark) + goroutine runtime (8 MiB default stacks, ids, counts, panic → exit 2, stack-overflow handler)
- [ ] 8.2 `go` statement codegen (eager evaluation of function + args)
- [ ] 8.3 Channels
  - [ ] unbuffered (direct hand-off)
  - [ ] buffered (ring buffer)
  - [ ] close semantics + panics
  - [ ] nil channels, `len`, `cap`, recv with `ok`
- [ ] 8.4 Directional channel types (same C++ type, distinct descriptors)
- [ ] 8.5 `select` (random poll order, lock order, default, nil cases, one entry per case + shared claim flag)
- [ ] 8.6 `for range ch`
- [ ] 8.7 `sync` + `sync/atomic`
  - [ ] Mutex / RWMutex (atomic state word, waiters outside the object, copyable)
  - [ ] WaitGroup (+ `Go`), Once, OnceFunc/OnceValue/OnceValues, Cond, Map, Pool
  - [ ] atomic typed API + `gp::atomic_ptr<T>` + function-style API (`std::atomic_ref`)
- [ ] 8.8 `time` (Go 1.23+ rules: unbuffered timer channels, no stale values, weak refs in the timer heap)
- [ ] 8.9 Exit semantics (flush + `_exit`, no static destructors, harness fails on TSan warnings)
- [ ] 8.10 `runtime` (Gosched, NumGoroutine, GOMAXPROCS remembers its value + cgroup-aware default, Goexit)
- [ ] 8.11 Deadlock detection (Go 1.23+ timer rule, foreign threads, `Goexit` message)
- [ ] 8.12 `-race` (TSan) CI job
- [ ] 8.13 Tests
  - [ ] ≥ 50 deterministic concurrency run tests
  - [ ] self-contained Go `test/chan` programs (from the pinned upstream source) as differential tests
  - [ ] stress: 10k goroutines, 1M channel ops; deep recursion
  - [ ] timer `cap`/`len`/`Stop`/`Reset` differential tests
- [ ] 8.14 (optional) `iter.Pull` on a helper thread
- [ ] 8.15 Thread layer for portability (`gp::os`: start with stack size, TLS, stack-overflow hook; Linux now, Windows in Phase 14)

## Exit criteria

- [ ] `workers` example passes on GCC and Clang
- [ ] Concurrency suite green under TSan (harness fails on any TSan warning)
- [ ] 10k simultaneous goroutines OK; deep recursion prints `fatal error: stack overflow`
- [ ] Deadlock messages (incl. `Goexit`) + exit code match Go
- [ ] Self-contained `test/chan` programs match Go output

## Decisions / notes

- 2026-10-13: one OS thread per goroutine. All blocking goes through `gp::waiter`.
- 2026-10-13 (your decision, Q4): OS threads only, no goroutine scheduler now or later. The
  optional fiber "v2" is removed from the plan; `runtime.LockOSThread` is a no-op that always
  holds; step 8.15 now prepares the thread layer for the Windows port.
- 2026-10-13 (audit): 8 MiB default stacks (small stacks saved no memory but crashed) + a
  stack-overflow handler; flush + `_exit` (with `_Exit`, TSan races exited 0); Go 1.23+ timer and
  deadlock rules; `gp::atomic_ptr` (libc++ lacks `atomic<shared_ptr>`); one select entry per case;
  only self-contained `test/chan` programs here. Now depends on Phase 07 (generic sync/atomic APIs).
