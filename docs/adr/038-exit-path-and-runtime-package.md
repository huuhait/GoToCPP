# ADR-038: The program ends with `gp::exit` (flush, leak check, `_exit`), and `runtime` is a Go package over a small native layer

Status: accepted (2026-10-18)

## Context

Steps 8.9 and 8.10. Goroutines are detached OS threads (ADR-007, ADR-026), so when `main` returns
other threads may still run, block in a system call or spin. C++'s ordinary exit (`std::exit`, a
return from `main`) runs the destructors of static objects and the thread-local destructors while
those threads still use the objects: the classic shutdown crash. Go runs nothing at exit. Separately,
a program that imports `runtime` needs `Gosched`, `NumGoroutine`, `GOMAXPROCS`, `Goexit`, `Caller`,
`Stack` and the rest, and the gate for `runtime` stood at Phase 09.

## Decision

- **One exit path: `gp::exit(code)`** (`goroutine.hpp`). `run_main` ends with `gp::exit(0)` after the
  panic settling of 8.1; `os.Exit` (Phase 09) and fatal errors call it too. It does `claim_death()` (a
  thread that is already ending the program with a panic's report keeps the report and its code; a
  second caller blocks), then `os::exit`: flush C stdio (`fflush(nullptr)`: what C and C++ code
  printf'd; gocpp's own output goes straight to the file descriptor with one `write`, nothing to
  flush), run the **leak check in an ASan build** (`__lsan_do_leak_check`, which `_exit` would skip),
  then `_exit(code)`. No static destructor, `atexit` function or thread-local destructor runs. Only
  one thread ever gets past the guard in `os::exit`, because the leak check must not run twice. `_exit`
  and not `_Exit`: sanitizers intercept `_exit`.
- **Exit never waits for other goroutines.** A goroutine blocked in a system call or on a channel, in a
  tight loop or starting threads, stops with the process. Measured: a program with 1000 blocked
  goroutines and one spinning goroutine takes about 0.11 s from start to exit, thread creation
  included.
- **Exit codes**: `main` returning is 0 whatever runs; a panic in any goroutine is 2 (8.1); `gp::exit(n)`
  is n. SIGPIPE on stdout or stderr kills the process with SIGPIPE, as in Go (already in the OS layer:
  `die_of_sigpipe`).
- **The harness fails a run whose stdout or stderr has `WARNING: ThreadSanitizer` or
  `FATAL: ThreadSanitizer`** (`tests/harness_test.go`, `runMismatches`), whatever the exit code and
  `// Stderr:` say. gocpp has no TSan mode for programs yet (8.12); the runtime's own C++ tests run under
  TSan (`TestRuntimePkgTSan`).
- **`runtime` is written in `.gp`** (`stdlib/runtime`, ported by hand from go1.25.14 with Go's
  documentation, `PORT.md` lists the differences) and its machine-dependent functions are
  `//gocpp:native` (ADR-027) functions of `gocpp/runtimepkg.hpp` in `gp::runtime`. This gives
  `runtime.Error`, `PanicNilError`, `TypeAssertionError`, `MemStats`, `Frames` and `Func` real Go types
  with Go's methods; the Phase 05 panics make their values before the package is loaded, so
  `init` of the package passes its own descriptors to the runtime (`gp::runtime::set_panic_values`) and a
  program's `r.(*runtime.PanicNilError)` accepts them. The gate row for `runtime` is Phase 08, with a row
  for `runtime/debug`; `runtime/cgo` stays at Phase 09 (9.2 owns `cgo.Handle`).
- **What the threads model changes**: `Gosched` is `sched_yield` (a hint: tests must not depend on the
  order Go's scheduler gives with `GOMAXPROCS(1)`); `LockOSThread` and `KeepAlive` do nothing;
  `GC` does nothing and `SetFinalizer` never runs the finalizer (reference counting, ADR-004);
  `GOMAXPROCS` remembers its value and nothing uses it, with Go 1.25's default (CPU affinity lowered by
  the cgroup CPU limit; read from `/proc/self/{cgroup,mountinfo}` like Go's `internal/runtime/cgroup`,
  so it is portable code over a small `os::cpu_limit`); `NumGoroutine` is `gp::num_goroutine()`;
  `Stack(buf, true)` writes the caller's stack only; `ReadMemStats` fills the heap numbers from the
  allocator (`mallinfo2`, or the sanitizer's allocator) and leaves the counters 0.
- **`Goexit`** is `gp::goexit()`: the Phase 05 signal, whose unwinding runs the deferred calls and where
  `recover()` returns nil. From the main goroutine the program goes on until the other goroutines have
  ended, then prints `fatal error: no goroutines (main called runtime.Goexit) - deadlock!` (exit 2); the
  detection of goroutines that are blocked forever is 8.11's. The registry (`num_goroutine`, the record
  of each goroutine, `num_blocked`) is what 8.11 needs and is unchanged.
- **`Caller`, `Callers`, `CallersFrames`, `FuncForPC`, `Stack`** read the program's own symbol table and DWARF
  line table (the reader that the panic trace uses: `trace.hpp`, `src/frames.cpp`), so files and lines are
  those of the `.gp` source and function names are Go's (`main.(*T).M`, `main.main.func1`, `main.Id[...]`).
  The runtime's own frames are not counted by `skip`.

## Consequences

- A destructor of a global C++ object never runs at exit (as in Go, where there are none); a
  C library that needs `atexit` to flush (a C++ `std::ofstream` global) loses its data: such code must
  flush itself (documented in `docs/spec/cpp-mapping.md`, "The runtime package and the end of the
  program").
- `GOOS`/`GOARCH` are constants of the Linux/amd64 build until other targets build programs.
- `ReadMemStats` and `Stack(all)` are approximations; a test that needs more would need real counters or a
  thread registry with stack capture (a signal per thread), neither of which is planned.
