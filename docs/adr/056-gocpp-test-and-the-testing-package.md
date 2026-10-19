# ADR-056: `gocpp test` generates a main package in the loader, and `testing` is Go's package on a few natives

Status: accepted (2026-10-18)

## Context

Phase 09 step 9.9 builds `gocpp test` and the standard package `testing`. When it was built, the packages
that Go's `testing` imports were still declaration stubs or not there at all: `flag`, `strings`, `regexp`,
`sync`, `time`, `os`, `runtime`, `context`, `io`. A test binary also needs a `main` package that lists the
tests, which `go test` generates and compiles together with the package under test, its test files (the
*test variant* of the package, in an import graph of its own) and its external test package (`x_test`).

## Decision

**The main package of a test binary is made by the loader.** `load.Config.Tests` (step 1.x) already loaded a
package with its `_test.gp` files in a graph of its own. `load.Config.TestMain` is called after the package
under test `p` and its external test package `xt` are loaded; it returns the source of a `package main` file,
which the loader reads (from `Config.Overlay`, named `_testmain.gp` in the folder of `p`), loads as the last
root with the import path `p.ImportPath + ".test"` and `ForTest` set, in the same graph: the file imports
`p`, so it gets the test variant, and it may import `xt` by its path and `testing/internal/testdeps`
(internal to `testing` for it only). `internal/gotest` scans the test files by syntax (Go's rules in
`cmd/go/internal/load/test.go`: `isTest`, the signatures `*testing.T`/`B`/`M`/`F`, `TestMain`, examples with
a last `// Output:` or `// Unordered output:` comment) and writes the source of the main package, which is
Go's template: `var tests = []testing.InternalTest{...}` and `testing.MainStart(testdeps.TestDeps{}, ...)`.
`cmd/gocpp` then builds it as any program (action graph, object cache), which is what makes one test binary
per package cost only the code of that package after the first run.

**`gocpp test` follows `go test`'s output rules** (`runTestActor` in `cmd/go/internal/test/test.go`), because
the exit criterion is that the output matches: streaming in local mode (no package argument) and with
`-bench`, buffering in package-list mode (the output of a passing package is dropped unless `-v` or `-list`),
`ok  \tpkg\t0.003s`, `FAIL\tpkg\t0.003s`, `?   \tpkg\t[no test files]`, `FAIL\tpkg [build failed]` (type
errors; `[setup failed]` for load errors and wrong test signatures), a final `FAIL` after several packages,
exit status 1, the backup alarm that kills a test binary after `-timeout` plus a minute, and the default
`-timeout 10m`. The flags go to the binary as `-test.name=value`.

**`testing` is Go's package, written in `.gp`** (`stdlib/testing`: `testing.gp` is `testing.go`, `benchmark.gp`,
`example.gp`, `match.gp`, `fuzz.gp`), so its output is Go's by construction: the names and numbering of
subtests, `=== RUN`/`=== PAUSE`/`=== CONT`/`=== NAME`, the buffering and indenting of logs, `t.Fatal` that
stops the test with `runtime.Goexit` (`gp::goexit`) after the deferred calls, cleanups, `T.Parallel` with real
goroutines and the barrier channels of Go's code, benchmarks with `b.N` and `b.Loop`. What the missing
packages gave it is replaced by a short list of natives (`runtime/include/gocpp/testing.hpp`,
`//gocpp:native`, ADR-027): the clock, the one recursive lock that guards the state of the tests, the alarm of
`-test.timeout`, the stack of the calling goroutine with its Go function names, files and lines (the same
tables that print a panic's trace) for the `file.gp:12:` prefix of `t.Log` and for `t.Helper`, the capture of
file descriptor 1 for the examples, the environment, temporary folders and the working directory. `Init` reads
the `-test.*` flags from the program's arguments itself (package `flag` is a stub). The regular expressions of
`-run`, `-skip`, `-list` and `-bench` go through `testdeps.TestDeps.MatchString`, a native `std::regex`
bridge: the pattern is translated from RE2's syntax to ECMAScript's (`runtime/src/testing_regex.cpp`) with Go's
error messages for what Go rejects (lookahead, backreferences, possessive repetition, bad escapes) and a clear
error for what it cannot translate (`\pL`, flags inside a pattern); step 9.4 replaces it with
`regexp.MatchString`, a change in `testing/internal/testdeps` only. (Done in step 9.4c, ADR-083: the bridge and
`runtime/src/testing_regex.cpp` are gone, and `testdeps` compiles the pattern with package `regexp` as Go's does.)

**Fuzz targets run their seed corpus.** Go calls the function given to `F.Fuzz` with `reflect.Value.Call`, which
gocpp's `reflect` (ADR-035) does not have. `internal/gotest` rewrites `f.Fuzz(fn)` in the test files, when `fn` is
a function literal or a function of the package whose parameters have the types of a fuzz target, to
`f.GocppFuzz(func(t *testing.T, vals []any) { fn(t, vals[0].(T0), ...) }, []any{*new(T0), ...})`; the types of
the seeds and of the files of `testdata/fuzz/FuzzXxx` are checked against the zero values. A call that cannot be
rewritten skips the target with a message. The fuzzing engine is not part of gocpp.

**The package of a test can be `main`.** The generated main package is named `testmain` and is the program
(`load.Package.IsProgram`); a package named `main` that a test binary imports is a library there (its entry
point is not generated, its `func main` is a function), as in `go test`.

**Parallel tests start in a fixed order.** Go's goroutines are scheduled in an order that is the same in almost
every run; gocpp's are threads. The parent of parallel subtests releases them one at a time in the order they
paused, and the tests that wait for a `-parallel` slot take it first come first served, so that the output does not
change from run to run.

**Allocations are counted where there is a counter.** The runtime has no allocator statistics, so a test binary
replaces the global `operator new` (a weak definition in `runtime/src/testing_alloc.cpp`; the build links the
`testing*.cpp` files of the runtime, `rtfiles.TestingSources`, only into a program that imports `testing`, so that
other programs keep their size and ASan's own `operator new`) and counts its calls while a benchmark or `AllocsPerRun` measures. The numbers are of the C++ heap,
not Go's (docs/guides/testing.md says how they can differ).

**The test binary runs in a process group of its own** (Unix): the backup alarm (SIGQUIT after the timeout plus a
minute, SIGKILL five seconds later) ends the processes the tests started with it, and `gocpp test` passes SIGINT,
SIGTERM, SIGHUP and SIGQUIT on to it. The file names of `t.Log` come from a DWARF line index that the runtime builds
once (`indexed_lines_of`), so a new call site costs a binary search.

## Alternatives

- **A C++ `testing` runtime** (a `gp::testing` library that the generated main calls): faster to write, but
  every rule of Go's output would be written again, and drift from Go is silent. The ported code is checked
  by the same fixtures that check `gocpp test` against real `go test`.
- **Generating the main package outside the loader** and loading it as a second program: the test variant of the
  package under test would be loaded twice (once in the graph, once for the main package), and two packages
  with the same import path in one program do not link.
- **Wait for the ports** (`sync`, `time`, `os`, `flag`, `strings`, `regexp`): step 9.9 would start after 9.8.
  The natives are small, named, and each one is the replacement of one Go function that a port later makes
  unnecessary (`libLock` is `sync.Mutex`, `libNow` is `time.Now`, ...).

## Consequences

- The `testing` package is a real package from step 9.9 (it was a stub); programs reach it with
  `GOCPP_GATE_PHASE=09`, like the rest of Phase 09, until the phase is done.
- Differences from Go's `testing` (listed in `stdlib/testing/PORT.md` and `docs/guides/testing.md`):
  `T.Context`, `T.Output`, `B.Context`, coverage, profiles, `-json`, `-shuffle`, `-race`, the fuzzing engine and the
  goroutine dump of a timeout are not there; the allocation numbers count C++ `operator new`; the regular
  expressions are translated to `std::regex` until `regexp` (9.4) is ported; the output of many parallel
  tests is in a fixed order that is not always Go's.
- A test file cannot use `flag` or `strings` until step 9.4 (`sync`, `time` and `os` are ported); a `TestMain`
  that returns exits with the status of `m.Run`.
- `package main` can be tested: the generated main package is the program, and the package under test is
  imported as a library.
- Replacing the natives is mechanical: when `sync`, `time`, `os` and `regexp` are real, `testing.gp` can be
  diffed against Go's file with the natives' names as the only differences.

## Evidence

`tests/gotest` runs `gocpp test` and real `go test` on the same fixture modules and compares the output
(standard output after the normalization of times, paths and benchmark numbers, and the exit status).

## See

[Phase 09 README §3 step 9.9](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md),
[ADR-027](027-native-std-functions.md), [docs/guides/testing.md](../guides/testing.md).
