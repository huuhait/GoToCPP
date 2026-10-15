# Test programs

`go test ./tests/...` runs the test programs of gocpp. It is one Go test binary
([harness_test.go](harness_test.go)): it builds `./cmd/gocpp` once into a temporary folder,
finds the test programs by folder and runs each one through that `gocpp` binary. It is part
of `go test ./...`, so `make test` and `make ci` run it too.

```sh
go test ./tests/...                        # every test program
go test ./tests/... -v                     # also list each test, and why it is skipped
go test ./tests/... -run 'Run/'            # only tests/run
go test ./tests/... -run 'Examples/hello'  # one example, with each compiler
go test ./tests/... -run 'Debug/hello'     # one debug test (line table and gdb)
go test ./tests/... -run Golden -update    # rewrite the golden snapshots
make test-programs                         # go test -count=1 -v ./tests/...
```

| Test | Folder | A test is | It passes when |
|---|---|---|---|
| `TestRun` | [run/](run/README.md) | a `.gp` program with an `// Output:` block at the end | `gocpp run` prints that output, with the expected exit code and stderr |
| `TestRunVsGo` | [run/](run/README.md) | the same tests, with `GOCPP_RUN_VS_GO=1` (below) | real Go prints that output, with that exit code and stderr |
| `TestErrors` | [errors/](errors/README.md) | a `.gp` file with `// ERROR "regexp"` (and `// WARNING "regexp"`) comments | `gocpp check` reports exactly those errors (and warnings), on those lines |
| `TestGolden` | [golden/](golden/README.md) | a `.gp` file and its `.cpp.golden` snapshot | `gocpp emit` prints the snapshot |
| `TestDiff` | [diff/](diff/README.md) | a `.gp` program that is valid Go | `gocpp run` and real Go print the same stdout (with a `// diff:` line, also stderr) and exit with the same code |
| `TestInterop` | [interop/](interop/README.md) | a run test that uses C/C++ libraries | like `TestRun`; skipped when `pkg-config` can't find a library |
| `TestDebug` | [debug/](debug/README.md) | a `.gp` program with `// gdb:` comments (stops with `print`, `bt` and `next`) | its binary's DWARF line table maps the program's code to the right `.gp` lines (and nothing else to the `.gp` file), and a gdb session in batch mode stops at the `.gp` lines and shows what the comments say; the gdb part is skipped when gdb is missing or may not trace programs (a failure with `GOCPP_REQUIRE_GDB=1`, as in `make ci`) |
| `TestExamples` | [examples/](../examples/README.md) | a folder `examples/<name>/` with `main.gp` and `output.txt` | `gocpp run . <args>`, in the example's folder, prints `output.txt` (see [below](#examples)) |

In `tests/<folder>/`, each `.gp` file directly in the folder is one test. Other files (the
READMEs, headers next to a test, snapshots) and subfolders are not tests. The subtests are
named after the files and the compilers: `TestRun/slices_append/g++`.

## Which tests run

Every test file starts with `// phase: NN`: the phase of the plan that makes the test pass,
with two digits (00 to 15). A test for a later phase than the last finished one
(`Phase` in [internal/version](../internal/version/version.go)) is skipped with
"needs Phase 03 (now: Phase 00)". So a test can be written before its feature, and a phase
is done when its tests pass. While you build phase NN, `GOCPP_TEST_PHASE=NN` runs the tests up
to that phase instead (`GOCPP_TEST_PHASE=01 go test ./tests/...`), and gocpp accepts the features of each
test's own phase in it (`GOCPP_GATE_PHASE`, below). A missing or malformed phase
line fails the test, and so does any other malformed line, even in a test that is skipped.

A test is also skipped, with a message, when:
- its `//go:build` line (at the top of the file, in every kind of test) excludes the target.
  The target is the machine that runs the tests: its `GOOS` and `GOARCH` hold, `unix` on a
  Unix system, and the release tags `go1.1` to `go1.25`; any other tag doesn't;
- it has `// display: required` (examples) and neither `DISPLAY` nor `WAYLAND_DISPLAY` is set;
- `pkg-config --exists` doesn't find a package of its `// pkg-config:` line (interop tests and
  examples).

Each runner logs how many tests it found, skipped (by reason) and found malformed (`-v` shows
the log).

## Sanitizers

`make ci-sanitize` is the sanitizer job: it runs the tests that run programs (`TestRun`,
`TestDiff`, `TestInterop`, `TestExamples`) with `GOCPP_FLAGS=-asan`, so every program is built
with AddressSanitizer, LeakSanitizer and UndefinedBehaviorSanitizer and fails on the first
report, and then the emitter's tests in `internal/cppgen` with the same sanitizers in
`GOCPP_TEST_CXXFLAGS`; with g++ and clang++, and `-parallel 2`. Every test must be clean, with one
exception: a test that leaks a reference cycle on purpose (reference counting never frees a cycle,
divergence D1 in [docs/spec/language.md](../docs/spec/language.md)) says so with a line in its
header,

```go
// phase: 03
// lsan: cycle
```

and then runs with `ASAN_OPTIONS=detect_leaks=0` (after the `ASAN_OPTIONS` of the environment),
so LeakSanitizer is off for that test only; AddressSanitizer and UBSan stay on. The line is
allowed in run, diff and interop tests, and in `internal/cppgen/testdata/emit`. It is set with or
without `-asan` (without, it changes nothing), so the command that a failing test shows has it.
Today two tests have it: `tests/run/closures_recursive.gp` and
`internal/cppgen/testdata/emit/closures.gp`, both a closure that calls itself through a variable.

## Compilers

Each run, interop, diff, golden, debug and example test runs once per C++ compiler, as the
subtests `g++` and `clang++`; gocpp gets the compiler in `GOCPP_CXX`. A compiler that isn't
installed is a skip with a message. In `make ci` it is a failure instead: `make ci` sets
`GOCPP_REQUIRE_CXX=g++,clang++`, and then every runner fails when one of those compilers is
missing (or left out with `GOCPP_CXX`). To pick one compiler, set `GOCPP_CXX`, or anchor the
subtest name: `-run 'Run//^g\+\+$'` (`clang++` contains `g++`).

The debug tests also need gdb. Without it, or where gdb may not trace programs (no `ptrace` in
some containers and sandboxes), their gdb part is a skip. `make ci` also sets
`GOCPP_REQUIRE_GDB=1`, which makes both a failure (a missing gdb fails every runner, like a
missing compiler).

## Examples

`TestExamples` runs each folder `examples/<name>/` that has a `main.gp`, as
[examples/README.md](../examples/README.md) describes, like
[tools/gen-example-output](../tools/gen-example-output) does with real Go:

- it runs `gocpp run . <args>` with the example's folder as the working directory, so the file
  names in `args.txt` (`testdata/poem.txt`) work;
- `args.txt` (optional) is one line of arguments, split on spaces, `stdin.txt` (optional) is
  the standard input, and `env.txt` (optional) has `KEY=VALUE` lines that are added to the
  environment (`GOCPP_SMOKE=1` for the GUI programs);
- the header of `main.gp` (its comment lines before the package clause) may have
  `// ExitCode: N`, `// pkg-config: …`, `// display: required` and a `//go:build` line;
- stdout must equal `output.txt` byte for byte, and the exit code `// ExitCode:` (default 0).
  Stderr is not compared; it is shown when the test fails.

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `GOCPP_CXX` | `g++` and `clang++` | test with this compiler only, for example `GOCPP_CXX=clang++` (a path works too) |
| `GOCPP_REQUIRE_CXX` | none (`g++,clang++` in `make ci`) | a comma list of compilers that must be tested: one that is missing fails the tests instead of being skipped |
| `GOCPP_REQUIRE_GDB` | none (`1` in `make ci`) | `1`: gdb must run the debug tests: a missing gdb, or a gdb that may not trace programs, fails the tests instead of skipping the gdb part |
| `GOCPP_FLAGS` | none | more flags for `gocpp run`, split on spaces, for example `GOCPP_FLAGS=-asan` for a sanitizer run |
| `GOCPP_TEST_PHASE` | `Phase` of internal/version | run the tests up to this phase (00 to 15), for example `GOCPP_TEST_PHASE=01` while you build Phase 01 |
| `GOCPP_GATE_PHASE` | set per test | not set by you: for gocpp's own development and tests, gocpp's phase gates accept the features up to this phase (`cmd/gocpp/gatephase.go`). For a test whose `// phase:` comes after `Phase` of internal/version (one that runs only with `GOCPP_TEST_PHASE`), the harness passes that test's phase to its gocpp commands, so the test can use the features of its phase before the phase is finished. A test of a finished phase gets none, so an error test still sees the gates of later phases. It never turns the gates of finished phases back on |
| `GOCPP_RUN_VS_GO` | none | `1`: `TestRunVsGo` builds each test in `tests/run` with real Go (`go build`, like a diff test) and checks the test's `// Output:` block (compared like `TestRun` compares), `// ExitCode:` and `// Stderr:` regexps against what the Go binary does, so that an expected output pasted from Go can't drift. It checks the tests of every phase, runs no C++ compiler, and skips a test whose code `go build` rejects (an error at a line of the program: gocpp-only code) and a test with a `// divergence: Dn` line (it shows a divergence of [docs/spec/language.md](../docs/spec/language.md), so Go does something else on purpose). Any other `go build` failure fails the test, and a run that compares no test fails too. Off, `TestRunVsGo` is a skip: `GOCPP_RUN_VS_GO=1 go test ./tests/... -run RunVsGo` |
| `GOCPP_TEST_TIMEOUT` | `2m` | the time limit of one command (one `gocpp run`, one `go build`), as a Go duration |

`make` passes them on: `GOCPP_CXX=g++ make test`. `GOCPP_REQUIRE_CXX= make ci` turns the
compiler requirement of `make ci` off, and `GOCPP_REQUIRE_GDB= make ci` the gdb requirement. A malformed value fails the tests with a message.

Each command that a test runs has that time limit, and it is also stopped 5 seconds before
go test's own `-timeout` (10 minutes by default) runs out, so that its test fails with a message
instead of the test binary panicking; the message says which limit it hit. A command that
prints more than 8 MiB is stopped too. On Linux each command runs in a process group of its own:
to stop a command, the group gets SIGTERM (so that gocpp can remove its work folder) and then,
2 seconds later, SIGKILL; when a command ends, whatever it left running is killed. So a hung test
program stops with its test. Ctrl-C (or a termination) stops all running commands as well,
also the ones that the parallel tests start while it does (they are killed at once), and
removes the temporary folder with the gocpp binary.
When a test fails, its message shows the command, ready to paste into a shell
(`cd examples/wc && GOCPP_CXX=g++ gocpp run . -l testdata/poem.txt`; `make build` puts gocpp
in `bin/`).

## Caching

`go test` caches the results. The harness tells it what the results depend on: the files in
`cmd/`, `internal/`, `runtime/` and `stdlib/` (the gocpp binary is built from them), the test
files, the examples and the variables above (also `DISPLAY` and `WAYLAND_DISPLAY`). It can't
see the compilers, gdb or the libraries: after you change those, run with `-count=1`. (Like any
flag outside go test's short list, `-update` turns the cache off by itself.)

## testdata/

[testdata/](testdata) holds the inputs of the harness's own unit tests
([format_test.go](format_test.go), for the readers of the formats in
[format.go](format.go)). It is never run as test programs, and `make lint` doesn't
gofmt-check it.
