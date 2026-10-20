# Testing with `gocpp test`

`gocpp test` builds and runs the tests of your packages the way `go test` does, and prints what `go test`
prints. A test is a Go test: the file ends in `_test.gp`, the function is `TestXxx(t *testing.T)`.

```go
// geom/geom_test.gp
package geom

import "testing"

func TestArea(t *testing.T) {
	if got := Area(3, 4); got != 12 {
		t.Errorf("Area(3, 4) = %d, want 12", got)
	}
}
```

```text
$ gocpp test
PASS
ok  	example.com/app/geom	0.004s
$ gocpp test -v -run Area ./...
=== RUN   TestArea
--- PASS: TestArea (0.00s)
PASS
ok  	example.com/app/geom	0.005s
?   	example.com/app	[no test files]
```

Programs that use `testing` need the Phase 09 gate while the phase is being built:
`GOCPP_GATE_PHASE=09 gocpp test`.

## What it builds

For each package that the arguments name (the package of the current folder without arguments, `./...`
for all of them), `gocpp test` builds one test binary:

- the package with its `_test.gp` files in the same package (internal tests: they see unexported names);
- its external test package, the `_test.gp` files with `package geom_test` (they import the package like
  any user does);
- a main package that `gocpp` generates (`example.com/app/geom.test`; `gocpp test -c` keeps the binary,
  `-work` the generated C++) and that lists the tests with `testing.MainStart`, like the one in `go test`.

The binary is cached like any program: only the code of the package is compiled again when it changes.
It runs in the folder of the package, with the arguments `-test.paniconexit0 -test.timeout=10m0s` and your
flags as `-test.NAME=VALUE`. A package that is a `package main` can be tested too (its `func main` is a function
like any other there, and its `init` functions and variables run); the main package of the test binary is the
generated one, named `testmain`, so the two do not meet.

A second `gocpp test` after you edited a `_test.gp` file or the package builds what changed and runs the new
test: the test binary goes through the object cache like any program (the up-to-date check of `gocpp build`
does not apply to it).

## What is a test

| Function | Is |
|---|---|
| `func TestXxx(t *testing.T)` | a test (`Xxx` does not start with a lower-case letter) |
| `func BenchmarkXxx(b *testing.B)` | a benchmark, run with `-bench` |
| `func FuzzXxx(f *testing.F)` | a fuzz target: its seed corpus runs as tests (see below) |
| `func ExampleXxx()` | an example; it runs when its last comment is `// Output:` or `// Unordered output:` |
| `func TestMain(m *testing.M)` | the entry of the test binary: call `m.Run()` (the exit status is taken from it) |

A `TestMain` is written as in Go, with what the standard packages it uses have by now. `flag` (Go's own package: `testing` registers the `-test.*` flags with `flag.CommandLine`, so `flag.Parse()` in a `TestMain` works and the program's own flags can follow `-args`: `gocpp test -args -n=3`; `testing.Short` and `testing.Verbose` panic before `flag.Parse`, as in Go), `strings`, `strconv`, `regexp`, `sort` and `os` work: `os.Exit(m.Run())` and `os.Args` build. The idiom
`os.Exit(m.Run())` is not needed: a `TestMain` that returns exits with the status of `m.Run` (as Go does since 1.15): a `TestMain` that returns exits with the status that `m.Run`
gave (as Go does since 1.15). `sync` (step 8.7), `time` (step 8.8), `os` and `fmt` work in tests. After the ports, nothing in `testing`
or in your tests changes.

### Fuzz targets

`go test` runs the seed corpus of every fuzz target: the entries of `f.Add(...)` and the files of
`testdata/fuzz/FuzzXxx/` (the format `go test fuzz v1`), each as a subtest `FuzzXxx/seed#0`, `FuzzXxx/<file>`.
`gocpp test` does the same, with the output of `go test`. Go calls the function that `f.Fuzz` receives with
`reflect`, which gocpp's `reflect` cannot do (no `Value.Call`), so `gocpp test` rewrites `f.Fuzz(...)` in the
test files before it builds them, when its argument is a function literal or the name of a function of the
package and its parameters have the types a fuzz target may have (`[]byte`, `string`, `bool`, `byte`, `rune`,
`float32`, `float64`, the sized integers). A fuzz target it cannot rewrite (for example `f` passed to a helper
that calls `f.Fuzz`) is skipped with a message that says why. `-fuzz` (the fuzzing engine that generates and
minimizes inputs) is not supported.

The signatures are checked as Go does, by their syntax: `func TestX(t *testing.B)` is
`wrong signature for TestX, must be: func TestX(t *testing.T)`, and the package fails with
`FAIL pkg [setup failed]`.

## Flags

| Flag | Meaning |
|---|---|
| `-v` | print every test as it runs (`=== RUN`, `--- PASS`) and the logs of the passing ones |
| `-run regexp`, `-skip regexp` | only the tests (and examples) whose name matches, or not; `A/b` matches subtests |
| `-count n` | run each test n times |
| `-short` | `testing.Short()` is true |
| `-failfast` | stop at the first failure |
| `-parallel n` | at most n parallel tests at a time (default: the number of CPUs) |
| `-cpu 1,2,4` | run with each of the CPU counts (names of benchmarks get the suffix `-N`) |
| `-timeout d` | the test binary panics (and prints the running tests) after d; 0 turns it off; default 10m |
| `-list regexp` | print the names of the matching tests, benchmarks, fuzz targets and examples; run nothing |
| `-bench regexp`, `-benchtime 1s` or `100x`, `-benchmem` | run the benchmarks |
| `-fullpath` | the file names of `t.Log` are full paths |
| `-c` | write the test binary (`pkg.test`, or the file of `-o`) and do not run it |
| `-o file` | write the test binary to the file (or into the folder, when it ends in `/` or exists); the test runs as well unless `-c` |
| `-asan`, `-nochecks`, `-x`, `-work`, `-cpperrors` | the build flags of `gocpp build` |
| `-args` | the rest of the arguments go to the test binary |

Another flag goes to the test binary, which reports the ones it does not know as `go test` does (`flag provided but
not defined: -bogus`, the usage of the test flags and of the program's own flags, exit status 2). Flags after `-args` are the program's own: define them with package `flag` and call `flag.Parse()` in a `TestMain` (a test binary without a `TestMain` parses the command line in `m.Run`, so an undefined flag stops it).

The patterns of `-run`, `-skip`, `-list` and `-bench` are Go's regular expressions (RE2), matched with package `regexp`
(`testing/internal/testdeps`, as in Go since step 9.4c): the same syntax, the same flags inside a pattern, Unicode classes, case
folding, and the same error texts (`testing: invalid regexp for element 0 of -test.run ...: error parsing regexp: ...`). The tests of
`tests/gotest` check many patterns against the test binary of real `go test` (the same names listed, the same error text). The
pattern is split at `/` for subtests. Go keeps the last compiled pattern; gocpp's `regexp` compiles slower, and a pattern with `/` asks for each of its elements for every subtest, so the test binary keeps up to 64 compiled patterns (and the error of a pattern that does not compile).

A limit of the platform, not of `-run`: a pattern with a large nested repeat such as `x{1,1000}y{1,1000}` is 2000 frames deep in `regexp`'s simplifier and compiler, and with `clang++ -asan` (whose stack frames are large) it overflows the 8 MB stack of the main goroutine and of any goroutine (goroutines are threads with a fixed stack); it is fine in the other builds, with g++ also under `-asan`.

## Output

The output is Go's, line by line (`tests/gotest` compares it with the output of real `go test`):

```text
=== RUN   TestA                       (with -v)
    a_test.gp:12: message             (the file and line of the t.Log call: t.Helper skips helpers)
--- FAIL: TestA (0.00s)
    --- FAIL: TestA/sub (0.00s)
FAIL
FAIL	example.com/app/geom	0.002s
```

As in `go test`, the output of a package that passes is shown in the local mode (no package argument), with
`-v`, `-list` and `-bench`; in the package-list mode (`gocpp test ./...`) it is only the `ok` line. A package
that fails shows everything it printed. After several packages with a failure there is a last line `FAIL`.
`?   \tpkg\t[no test files]` is a package without `_test.gp` files; `FAIL\tpkg [build failed]` is a package
whose tests do not compile (the errors are printed above it, with the header `# pkg [pkg.test]`).
The exit status is 1 if anything failed, 2 for a usage error.

A syntax error in a `_test.gp` file is `FAIL\tpkg [setup failed]` with the package's header `# pkg`, as in Go (go
finds it when it lists the package); in another file it is `[build failed]`. `go test` also runs `go vet` before it
builds the tests, and stops on what vet finds (for example a wrong `f.Add` type); `gocpp test` does not run a vet
yet (`-vet=off` is accepted and ignored).

A test that panics prints Go's `--- FAIL`, the panic and the trace, and the binary exits with status 2.
`t.Fatal` and `t.FailNow` stop the test with `runtime.Goexit`: its deferred calls and `t.Cleanup` functions run.

## The `testing` package

It is Go 1.25's package, written in `.gp` (`stdlib/testing`), with:

- `T`: `Error`, `Errorf`, `Fatal`, `Fatalf`, `Log`, `Logf`, `Skip`, `Skipf`, `SkipNow`, `Skipped`, `Fail`,
  `FailNow`, `Failed`, `Helper`, `Cleanup`, `TempDir`, `Setenv`, `Chdir`, `Name`, `Attr`, `Deadline`, `Run` (subtests
  named `TestX/sub`, with `#01` for a repeated name) and `Parallel` (real goroutines, limited by `-parallel`);
- `B`: the same, `N`, `Loop` (Go 1.24), `Elapsed`, `ResetTimer`, `StartTimer`, `StopTimer`, `SetBytes`,
  `ReportAllocs`, `ReportMetric`, `Run`, `RunParallel` with `PB.Next`, `SetParallelism`; `testing.Benchmark`,
  `BenchmarkResult` with `time.Duration`;
- `F`: `Add` and `Fuzz` for the seed corpus (above);
- `M` and `TestMain`, `testing.Short`, `Verbose`, `Testing`, `Init`, `Main`, `MainStart`, `RunTests`,
  `RunExamples`, `RunBenchmarks`, `TB`, `InternalTest` and the other internal types.

A benchmark line is `BenchmarkAdd-8   \t 1000000\t      1234 ns/op`; `b.Loop()` and `b.N` loops work.

### What differs from Go

| Go | gocpp |
|---|---|
| `T.Context`, `B.Context`, `T.Output` | not there yet (they need `context` and `io`) |
| `-fuzz` (generating inputs), `F.Fuzz` with an argument that is not a function literal or a function of the package | not supported; the seed corpus runs (above) |
| `B/op` and `allocs/op` of benchmarks, `testing.AllocsPerRun` | the calls of C++ `operator new` while measuring (`runtime/src/testing_alloc.cpp`), not Go's allocations: `make([]byte, 1024)` is 1 allocation here and in Go, but a value that Go's compiler keeps on the stack can cost one here and a closure that Go heap-allocates may not; the bytes include the runtime's own block header; `AllocsPerRun` does not set `GOMAXPROCS(1)`, so a goroutine that allocates at the same time is counted |
| `-race`, `-cover`, `-json`, `-shuffle`, profiles, `-vet` | not supported (the flags say so; `-shuffle=off` and `-vet` are accepted) |
| the order of `=== CONT` and `--- PASS/FAIL` lines of parallel tests | Go's scheduler runs the goroutines of the parallel subtests of a test in an order of its own that is the same in almost every run (and not the order of creation for many tests); gocpp's goroutines are threads, so the parent releases the parallel tests one at a time in the order they paused, waits up to 0.3 ms for each to finish, and those that have to wait for a slot (`-parallel n`) take it in that order: the same output in every run, equal to Go's in the usual small cases and different from Go's in the order of many tests |
| the goroutine dump when a test times out (`panic: test timed out after 10m0s`, `running tests:` and then every goroutine's stack) | the first lines are Go's; the dump of all goroutines is missing (the runtime has no SIGQUIT handler), and the backup alarm of `gocpp test` (SIGQUIT after the timeout plus a minute) just ends the process |
| a test file name in `t.Log` is the `.gp` file | it is (`geom_test.gp:12:`) |

On Windows (`gocpp test` natively, or a test binary from `gocpp test -c -os windows`) the `testing` package behaves
as Go's does there: `T.TempDir` paths use backslashes and the folders are in `%TMP%`/`%TEMP%` (read-only files in them
are removed too), the names of `T.Setenv` variables are case insensitive and the C library's copy of the environment
follows, `T.Chdir` does not set `PWD`, an example's output is compared with `\r\n` read as `\n` (as in Go), and the `cpu:` line of a benchmark is the processor name from the registry.
These are tested on a real Windows PC (`tests/windows_gotest_test.go`).

The test binary runs in a process group of its own (Unix); the signals that `gocpp test` gets (SIGINT from Ctrl-C,
SIGQUIT, SIGTERM, SIGHUP) are passed on to it and its group, and the processes that the tests started are killed
when the binary is gone after such a signal, and when the backup alarm ends a test binary that does not stop (a
background job of a shell ignores SIGINT, so the signal alone would leave it).

With `-asan`, the test binary runs with `ASAN_OPTIONS=detect_leaks=0` unless you set `ASAN_OPTIONS` yourself:
memory is reference counted, and a cycle of values the program drops is a leak that LeakSanitizer reports at
exit (the repository's own tests turn it off too). The cycles inside the `testing` package are broken at the end
of each test.

`stdlib/testing/PORT.md` lists the files and the natives behind these differences.
