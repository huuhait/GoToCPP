# runtime, runtime/debug

Ported by hand from go1.25.14's `src/runtime` (`extern.go`, `mprof.go`, `mstats.go`, `panic.go`, `proc.go`,
`symtab.go`, `error.go`, `debug.go`) and `src/runtime/debug` (BSD-3-Clause, see `../LICENSE`) in Phase 08 step 8.10
(ADR-038). Only the part of the package that gocpp has is here, with Go's documentation. The functions that need
the machine are `//gocpp:native` (ADR-027) and live in `runtime/include/gocpp/runtimepkg.hpp` and
`runtime/src/{runtimepkg,frames}.cpp`. Phase 09's `tools/portstd` takes the port over: these are the changes it must
keep.

What is different (a goroutine is an OS thread, memory is reference counted, ADR-004 and ADR-007):

- `Gosched` is `sched_yield`; `LockOSThread`, `UnlockOSThread` and `KeepAlive` do nothing; `GC` does nothing;
  `SetFinalizer` never runs the finalizer; `NumCgoCall` is 0.
- `GOMAXPROCS` remembers its value and nothing uses it. The default is Go 1.25's: the CPUs the process may run on
  (`sched_getaffinity`), lowered by the control group's CPU limit (cgroup v2 `cpu.max`, v1 `cpu.cfs_*`; rounded up,
  at least 2), and the `GOMAXPROCS` environment variable overrides it. `SetDefaultGOMAXPROCS` (Go 1.25) restores the
  default and ignores the variable.
- `Goexit` is `gp::goexit()` (the Phase 05 `goexit_signal`): the deferred calls run, `recover()` returns nil in
  them. From the main goroutine the program goes on until the other goroutines have ended, then
  `fatal error: no goroutines (main called runtime.Goexit) - deadlock!` (step 8.11 detects it when they are blocked
  forever instead of ending).
- `Caller`, `Callers`, `CallersFrames`, `FuncForPC` and `Stack` read the program's own debug information (the
  symbol table and the DWARF line table, as a panic's stack trace does). Names are Go's (`main.f`,
  `main.(*T).M`, `main.main.func1`, `main.Id[...]` for an instance of a generic function), files and lines are those
  of the `.gp` source. Inlined frames are not expanded; `Func.Entry` is the address of the C++ function.
- `Stack(buf, true)` writes the calling goroutine only (the other goroutines are threads, and their stacks are not
  known).
- `ReadMemStats` fills `Alloc`, `HeapAlloc`, `HeapInuse`, `Sys`, `HeapSys` and `HeapIdle` from the
  allocator (`mallinfo2`, or the sanitizer's allocator in a sanitizer build); every other field (the counts of allocations,
  frees and collections, `TotalAlloc`, `HeapReleased`, the pauses) stays 0 because nothing counts them.
- `MemProfileRate`, `SetBlockProfileRate`, `SetMutexProfileFraction` and `SetCPUProfileRate` keep their values; no
  profile is made.
- `TypeAssertionError` and `PanicNilError` are the types of the panics that the runtime makes. The runtime makes
  them before the package is loaded (a program that does not import `runtime` never names them), and `init` of the
  package hands it the package's own values (`gp::runtime::set_panic_values`) so `r.(*runtime.PanicNilError)` works.
- `GOOS` and `GOARCH` are the constants `linux` and `amd64`, whatever the target: a Phase 15 task when other targets
  build programs.
- `Pinner` does nothing (memory does not move).

Not built: `AddCleanup`, `Breakpoint`, `CPUProfile`, `MemProfile`, `BlockProfile`, `MutexProfile`, `GoroutineProfile`,
`ThreadCreateProfile`, `StackRecord`, `MemProfileRecord`, the trace and metrics functions. `runtime/debug`: `ReadBuildInfo`, `ReadGCStats`, `SetCrashOutput`, `WriteHeapDump`
and `SetMemoryLimit`'s enforcement (the limit is stored) come with Phase 09 or are never built.
