# Phase 08: Concurrency · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] 8.1 `gp::waiter` (park/unpark) + goroutine runtime (8 MiB default stacks, ids, counts, panic → exit 2, stack-overflow handler)
- [x] 8.2 `go` statement codegen (eager evaluation of function + args)
- [x] 8.3 Channels
  - [ ] unbuffered (direct hand-off)
  - [ ] buffered (ring buffer)
  - [ ] close semantics + panics
  - [ ] nil channels, `len`, `cap`, recv with `ok`
- [x] 8.4 Directional channel types (same C++ type, distinct descriptors)
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
- [x] 8.15 Thread layer for portability (`gp::os` in `runtime/src/os/`: start with stack size, TLS, stack-overflow hook; Linux now, Windows in Phase 14)

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
- 2026-10-17: Jobs 8.15 and 8.1 merged (three review rounds).
  Decisions: (1) a goroutine is a detached OS thread with an 8 MiB stack; `gp::waiter` has permits (unpark before park is not lost);
  (2) generated programs get `-fstack-clash-protection` (debug flags; runtime cache key changes once): without it a frame bigger than
  the 256 KiB guard jumps it and dies with a bare SIGSEGV instead of 'stack overflow'; (3) the death of a goroutine is claimed by ONE
  thread; the others wait at most 30 s, then exit 2 silently; (4) `current_goroutine_id` reads a thread_local slot (async-signal-safe
  only while the runtime is linked into the executable, not a shared library); (5) the overflow report prints `goroutine N [running]:`
  but not Go's `runtime stack:` section or a trace; stdio is not flushed in the handler (documented); (6) thread tests end with
  `threads_ended()` (runtime/test/threads_ended.hpp): every new test that starts threads must too (LeakSanitizer vs /proc/self/task);
  (7) gaps for later steps are in TODO.md: 8.11 needs a live-goroutine registry and blocked count, 8.10, 8.2.
- 2026-10-18: Jobs 8.2, 8.3, 8.4 merged after two review rounds (ADR-026). Decisions: (1) THREAD REUSE: a finished goroutine's thread waits up
  to 5 s for the next `go` (<=256 idle, default stack only); ids/records never reused; ASan builds never let a thread exit
  (LeakSanitizer); a reused thread starts clean (tested); (2) panics: `defer close(done); panic(..)` in a goroutine exits 2 like Go:
  `run_main` calls `detail::settle_panics()`: waits <=200 ms (injectable `set_settle_timeout`) while `panicking()>0` and no death
  is claimed, then blocks main if a death was claimed; the count follows a `panic_token` inside `panic_error`; (3) `gp::waiter` is a
  pointer to a state in src/waiter.cpp and the channel object lives in src/chan_core.hpp so rt.hpp stays light (+0 compile time);
  atomics are compiler builtins in headers; (4) the blocked counter (`num_blocked`) is decremented by the WAKER in `wake()` /
  `detail::unblock(g)` exactly once: 8.5 (select) and 8.8 (time) MUST park through `block_scope` under the lock and wake through
  `wake()`/`unblock`; 8.11's check can use `num_blocked()==num_goroutine()`; (5) scale on this machine: 1k 0.10 s, 10k 0.76 s/66 MB,
  100k 13 s/1.3 GB; a default Linux (max_map_count 65530, kernel <6.13) stops at ~20-30k threads; the real fix would be an M:N
  scheduler (rejected by ADR-007): REVISIT in Phase 15 if programs with 100k goroutines matter; (6) channel latency ~1 us per
  hand-off (Go 0.2 us): a futex-based waiter could cut it; (7) a deadlock (`<-ch` with no sender) hangs until 8.11; (8) gates:
  featGo/featChannels lift when Phase 8 is finished (featChannels also covers `range` over a channel: 8.6 first), featSelect with
  8.5; tests run with GOCPP_TEST_PHASE=08; (9) gaps: no `created by` line in goroutine traces; `make(chan struct{}, n)` allocates
  nothing now (zero-size); `%T` of a channel in the fmt shim prints `chan T` for all directions until 5.11 (descriptors know);
  idle threads keep their touched stack pages up to 5 s (documented); a goroutine panic racing main's return follows Go (main wins
  only if the deferred call is slower than the bound).
