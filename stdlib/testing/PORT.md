# testing: how this package differs from Go 1.25's

`testing.gp` is Go's `testing.go`, `benchmark.gp` is `benchmark.go`, `example.gp` is `example.go` and
`run_example.go`, `match.gp` is `match.go`, `fuzz.gp` is `fuzz.go` (the seed corpus only: no fuzzing engine).
`testing/internal/testdeps` is Go's package with a native regular expression matcher.

## The natives (`runtime/include/gocpp/testing.hpp`, ADR-056)

Their C++ is in `runtime/src/testing*.cpp` and `runtime/src/os/<os>/testing*.cpp` (`rtfiles.TestingSources`), which the
build compiles and links only into programs that import `testing`: `testing_alloc.cpp` replaces the global `operator
new`, and a program that does not test must keep the system's (and ASan's), and its size.

| Native | Replaces | Until |
|---|---|---|
| `libLock`, `libUnlock` | `sync.Mutex`, `sync.RWMutex`, `sync/atomic` (one recursive lock) | 8.7 is merged: `sync.Mutex` can replace it (the lock is recursive and one for all tests; Go has a mutex in each test), a later cleanup |
| `libNow`, `libSleep`, `libAlarmWait`, `libAlarmCancel` | `time.Now`, `time.Sleep`, `time.AfterFunc` (a monotonic clock in nanoseconds; `time` itself is merged since 8.8, but the testing package keeps its int64 clock: it is cheaper and `T.Deadline` converts) | later cleanup |
| `libFrames` | `runtime.Callers`, `runtime.CallersFrames` (on `gp::detail::shown_frames` and `resolve_pcs` of `frames.cpp`, the runtime's reader of the program's symbols and DWARF line table, which keeps a line index: a new call site costs a binary search) | 8.10 is merged: `runtime.Callers` can replace it when `runtime` has the frames of the Go names |
| `libGoexit` | `runtime.Goexit` (it is `gp::goexit`: stays) | |
| `libLookupEnv`, `libSetEnv`, `libUnsetEnv`, `libGetwd`, `libChdir`, `libMakeTempDir`, `libMakeDir`, `libRemoveAll`, `libReadFiles`, `libReadFile` | `os` | 9.6 is merged: `os` can replace them |
| `libCaptureStart`, `libCaptureStop` | `os.Pipe`, `io.Copy` on `os.Stdout` (examples) | 9.6 |
| `libExit`, `libArgs`, `libProgramName`, `libNCPU`, `libGOOS`, `libGOARCH`, `libCPUName` | `os.Exit`, `os.Args`, `runtime.GOMAXPROCS`, `runtime.GOOS`, `runtime.GOARCH`, `internal/sysinfo` | 9.6 |
| `libParseFloat` | `strconv.ParseFloat` (corpus files of fuzz targets) | strconv |
| `libAllocCounting`, `libAllocs` | `runtime.ReadMemStats` (`Mallocs`, `TotalAlloc`) | the runtime has no allocator counters; here they count the calls of C++ `operator new` (`runtime/src/testing_alloc.cpp`) |
| `testdeps.matchString` | `regexp.Compile` and `MatchString`, as in Go: done in step 9.4c (the `std::regex` bridge, `runtime/src/testing_regex.cpp`, is gone) | |

## Left out or changed

- The flags are Go's: `Init` registers them with package `flag`'s command line (the same names, types and usage texts; `-test.v` takes `test2json`,
  `-test.benchtime` a duration or `Nx`) and `m.Run` calls `flag.Parse` unless the `TestMain` did, so `flag.Parse()` in a `TestMain` and
  `gocpp test -args -n=3` work for a program's own flags, and an undefined flag gives package `flag`'s message, usage and exit status 2.
  `Short` and `Verbose` panic before `flag.Parse`, as in Go. The flags that need nothing from the machine are accepted and ignored (profiles, coverage, fuzz, trace); `-test.shuffle` other than `off` exits with 2.
- Strings helpers of `util.gp` replace `strings`, `strconv`, `slices` and `unicode`.
- Not ported: `T.Context`, `B.Context`, `T.Output` (`context`, `io`), `testing.Coverage`, `testing.CoverMode` (always
  ""), `testing.Cover`, the race detector hooks, `-test.json`, `-test.shuffle`, `-test.testlogfile`, the fuzzing
  engine (`-test.fuzz`, minimization, `F.Fuzz` with an argument that is not a function literal or a function
  of the package: it is skipped), `T.Parallel`'s `AllocsPerRun` check, and the goroutine dump of a timeout.
- `F.Fuzz`: Go calls the fuzz function with `reflect`. `internal/gotest` rewrites `f.Fuzz(fn)` to
  `f.GocppFuzz(wrapper, zeros)` in the test files (`internal/gotest/fuzz.go`; `fuzz.gp` explains it).
- `testing.AllocsPerRun`, `-benchmem`, `ReportAllocs`: C++ `operator new` calls and bytes while measuring, not Go's
  heap allocations (docs/guides/testing.md).
- The order of the parallel tests: `T.Parallel` waits on a channel of its own that the parent test sends to, in
  the order the tests paused (Go closes one channel for all of them and its scheduler starts them in an order
  that is the same in almost every run); `testState` keeps the tests that wait for a `-parallel` slot in a
  first-come first-served list (Go: the order in which the receivers of a channel were queued).
- `frameSkip` (the line that `t.Log` prints) is Go's: the cleanup stack (`cleanupName`, `cleanupPc`) and the
  creator stack of subtests (`creator`) are kept as lists of frames (`callers`) instead of program counters.
- References that make a cycle (a test and its writer, a test and its parallel subtests) are cleared when the
  test is done: memory is reference counted.
- `testing.Main`'s `deps` is Go's `testDeps` reduced to what the generated main needs (`ImportPath`,
  `MatchString`, `SetExitCode`).
