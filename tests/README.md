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
| `TestRace` | [race/](race/README.md) | a `.gp` program that has a data race (`race_*`) or has none (`ok_*`) | `gocpp run -race` ends with exit code 66 and Go's report on stderr, or runs silent with exit code 0 (Linux only; with the first compiler only, unless `GOCPP_RACE_COMPILERS=all`, which `make ci-race` sets) |
| `TestRaceVsGo` | [race/](race/README.md) | the same programs, with `GOCPP_RUN_VS_GO=1` | real Go's race detector (`go build -race`) gives the same output, exit code and report lines |
| `TestErrors` | [errors/](errors/README.md) | a `.gp` file with `// ERROR "regexp"` (and `// WARNING "regexp"`) comments | `gocpp check` reports exactly those errors (and warnings), on those lines |
| `TestGolden` | [golden/](golden/README.md) | a `.gp` file and its `.cpp.golden` snapshot | `gocpp emit` prints the snapshot (all the files of the program, each after a line `//// <path>`) |
| `TestDiff` | [diff/](diff/README.md) | a `.gp` program that is valid Go | `gocpp run` and real Go print the same stdout (with a `// diff:` line, also stderr) and exit with the same code |
| `TestInterop` | [interop/](interop/README.md) | a run test that uses C/C++ libraries, or an interop error test (`// ERROR` comments) | like `TestRun`; a folder with a `main.gp` is a test too (a package with its own C/C++ files, run as `gocpp run .`); skipped when `pkg-config` can't find a library. An error test passes when `gocpp build` fails with exit code 1 and reports exactly those errors (the C++ compiler's and the linker's, at the `.gp` lines), and no other output: the line "(and N more errors in gocpp's code ...)" is an error too, unless the file says `// others-ok: N reason` (exactly N of them) |
| `TestDocSnippets` | [docs/spec/interop.md](../docs/spec/interop.md), [interop-cookbook.md](../docs/spec/interop-cookbook.md) | every `<!-- test: PATH -->` block of the two docs | the fenced block after the marker equals the file (without its `// phase:` header lines and `// Output:` block; `<!-- output: PATH -->` shows the output), every `cookbook_*` and `doc_*` interop test is shown in the docs, an output marker has its test marker before it, a shown program that prints has its output shown, every ```go block carries a test marker or `<!-- illustration: reason -->`, and every relative link of the two docs leads to a file and a heading that exist |
| `TestZoo` | [zoo/](zoo/README.md) | the header zoo: one `.gp` program per header or library (the C and C++ standard headers, sqlite3, zlib, SDL2, raylib), with an `// Output:` block | `gocpp check` is clean, and `gocpp run` prints that output with each compiler. A file with `// display: required` (SDL2, raylib) is only built, never run |
| `TestPkgs` | [pkgs/](pkgs/README.md) | a folder `pkgs/<name>/` with a `gocpp.mod`, a `main.gp` (with `// Output:`) and the packages it imports | `gocpp run .` prints that output, like `TestRun`; a program of several packages (Phase 06) |
| `TestPkgsVsGo` | [pkgs/](pkgs/README.md) | the same folders, with `GOCPP_RUN_VS_GO=1` | real Go prints that output (the module is copied with `.gp` renamed to `.go`) |
| `TestPkgErrors` | [pkgs/errors/](pkgs/README.md#errors) | a module folder with `// ERROR` comments in any of its `.gp` files | `gocpp check -e ./...` reports exactly those errors, on those lines |
| `TestDebug` | [debug/](debug/README.md) | a `.gp` program with `// gdb:` comments (stops with `print`, `bt` and `next`) | its binary's DWARF line table maps the program's code to the right `.gp` lines (and nothing else to the `.gp` file), and a gdb session in batch mode stops at the `.gp` lines and shows what the comments say; the gdb part is skipped when gdb is missing or may not trace programs (a failure with `GOCPP_REQUIRE_GDB=1`, as in `make ci`) |
| `TestGDBPrinters` | [gdb/](gdb/README.md) | a program `gdb/NAME.gp` with `// @tag` lines, and a session `NAME.gdb` (gdb commands) with the output that gdb must print (`NAME.golden`, or `#want` regexps) | gdb's output for each command, with the printers that every program carries (`runtime/debug/gocpp-gdb.py`): Go values, Go names in backtraces, the goroutine list; built at `-O0` and `-O2`, with trimmed paths, and with `-asan`; skipped like `TestDebug` |
| `TestLLDBPrinters` | [gdb/](gdb/README.md) | a session `gdb/NAME.lldb` for the program `gdb/NAME.gp` | the same for lldb and `runtime/debug/gocpp_lldb.py` (best effort); skipped when lldb is missing |
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
- it has a `// zoneinfo: required` line (run tests: a program that loads time zones by name with `time.LoadLocation`) and the machine has no time zone database with `America/New_York`, `Europe/Berlin` and `Asia/Kolkata` (the `ZONEINFO` variable, else `/usr/share/zoneinfo`, `/usr/share/lib/zoneinfo`, `/usr/lib/locale/TZ`, `/etc/zoneinfo`);
- it has a `// pending: reason` line (below the phase line; allowed in run, diff, interop and pkgs tests and in
  examples) and `GOCPP_TEST_PENDING` is not `1`. It says that the test's phase is only partly built and
  the test fails until a named step lands: `make ci` runs the tests up to Phase 10 (`GOCPP_TEST_PHASE=10`,
  below), and the examples `generics`, `workers`, `grep-lite`, `macros`, `must-errors` and `wc` are pending. The step that makes the test pass removes
  the line; `GOCPP_TEST_PENDING=1` runs the pending tests meanwhile. A `// pending:` line in a test of a finished phase
  (`// phase:` up to `Phase` of internal/version) fails the test, because it would hide a real failure, and
  `TestPendingFiles` lists the pending tests (a short list: it names each one), so hiding another one needs an edit there;
- its `//go:build` line (at the top of the file, in every kind of test) excludes the target.
  The target is the machine that runs the tests: its `GOOS` and `GOARCH` hold, `unix` on a
  Unix system, and the release tags `go1.1` to `go1.25`; any other tag doesn't;
- it has `// display: required` (examples) and neither `DISPLAY` nor `WAYLAND_DISPLAY` is set: an example is then
  still **built** with each compiler (its binary is not run), so a GUI example (raylib) cannot rot unseen;
- `pkg-config --exists` doesn't find a package of its `// pkg-config:` line (interop tests and
  examples).

Each runner logs how many tests it found, skipped (by reason) and found malformed (`-v` shows
the log).

## Tests of the standard library's internal packages

A run test may say `// std-internal: reason` under its phase line when it tests a package that
only the standard library may import (`internal/bytealg`, `internal/byteorder`, ...: Go's rule, which gocpp keeps for
programs). The harness then runs that test's gocpp commands with `GOCPP_ALLOW_INTERNAL=1`, which opens the internal
packages of the standard library that gocpp has (`load.Config.AllowStdInternal`): a module's `internal` folders, and a
standard package that does not exist, stay closed. No other test gets the variable, so users keep seeing "use of
internal package ... not allowed"; a test of another kind with the line is malformed. (A pkgs test may say it too, and gets the variable; `TestPkgsVsGo` has no twins.) For `TestRunVsGo` real Go cannot
import `internal/...` either: the harness copies the `.gp` files of the packages the test imports as `.go` files into a
module next to the test (`difftest/internal/<name>`, [twin_test.go](twin_test.go)), and builds the program there. The
natives of `internal/bytealg` (C++ in gocpp) are replaced in that copy by calls of `bytes` and `strings`
(`testdata/twin/bytealg_native.go.txt`), so a test's `// Output:` block must equal what Go computes for the same inputs.

## Sanitizers

`make ci-race` is the race detector job (step 8.12, [docs/guides/race.md](../docs/guides/race.md)): the
programs of [race/](race/README.md) with both compilers, and every test of `tests/run` that starts a
goroutine or uses a channel or `select` with `GOCPP_FLAGS=-race`, where a report fails the test (the
tests expect no stderr). `-race` and `-asan` can't be combined.

`make ci-sanitize` is the sanitizer job: it runs the tests that run programs (`TestRun`,
`TestDiff`, `TestInterop`, `TestZoo`, `TestExamples`) with `GOCPP_FLAGS=-asan`, so every program is built
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
Today 17 tests have it (`rg -l "^// lsan: cycle" tests internal`); the first ones were `tests/run/closures_recursive.gp` and
`internal/cppgen/testdata/emit/closures.gp`, both a closure that calls itself through a variable,
`tests/run/embed_later_types.gp`, two structs that embed pointers to each other, and
`tests/run/sync_cond_queue.gp` and `sync_cond_semaphore.gp`, a struct that holds a `sync.Cond` whose
Locker is a pointer to a field of that same struct (the usual `sync.NewCond(&q.mu)`); the others are later tests of the same kinds, and `derive_cycle.gp` (a value that points to itself).

## What the tests assume of the host

gocpp's targets are little-endian (x86-64, arm64), and the interop tests rely on it: `c_unions.gp`
reads the bytes of a union. The tests that print the `what()` of a standard exception
(`cxx_exception_stoi`, `callback_recover`, `cookbook_exceptions`) expect libstdc++'s text
(`stoi`; libc++ says `stoi: no conversion`), which both compilers of the Nix shell use. A host
whose clang++ uses libc++ would see those lines differ.

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
| `GOCPP_TEST_PENDING` | none | `1`: also run the tests with a `// pending:` line (they fail until their step lands) |
| `GOCPP_TEST_TIMEOUT` | `2m` | the time limit of one command (one `gocpp run`, one `go build`), as a Go duration |
| `GOCPP_TEST_TARGET` | none | `windows`: also build the run tests for windows/amd64 and run them under Wine (`TestRunWindows`, `TestWindowsPrograms`, `TestWineIsolation`; the section "Windows under Wine" below); run on a real PC with `GOCPP_WINDOWS_SSH` |
| `GOCPP_TEST_WINDOWS` | `sample` | with `GOCPP_TEST_TARGET=windows`: `sample` runs the tests of `windows_sample.txt`, `all` every run test that `windows_skip.txt` does not name |
| `GOCPP_WINDOWS_LIBS` | none | with `GOCPP_TEST_TARGET=windows`: a folder with `include/` and `lib/` of sqlite3, zlib, SDL2 and raylib built for MinGW-w64 (`tools/winlibs.sh DIR`); `TestWindowsPrograms/lib*` skip without it |
| `GOCPP_WINDOWS_SSH` | none | `user@host`: the Windows tests (`GOCPP_TEST_TARGET=windows`) run their programs on that real Windows PC over ssh instead of under Wine (key authentication; `internal/winssh`; the variable is the only place that names the machine; never commit it) |
| `GOCPP_WINE_JOBS` | `4` | the programs that run under Wine at once (Wine's server is one process) |
| `GOCPP_REQUIRE_WINE_OVERFLOW` | none | `1`: `TestWindowsPrograms/overflow` fails instead of skipping when this machine's Wine ends a stack overflow with exit code 1 and no message |
| `GOCPP_REQUIRE_WINE` | none | `1`: a missing `wine` or MinGW-w64 compiler fails the Windows tests instead of skipping them |

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

## Windows under Wine

`GOCPP_TEST_TARGET=windows` turns on the tests that build for `windows/amd64`
(`gocpp build -os windows`, MinGW-w64 from the dev shell, or the pinned llvm-mingw when none is in
`PATH` and the machine can download it) and run the `.exe` files under Wine
([docs/guides/windows-cross.md](../docs/guides/windows-cross.md), ADR-055):

- `TestRunWindows` runs the run tests of `windows_sample.txt` (OS-independent: goroutines, channels,
  select, panics and their exit codes, defer and recover, maps, strings, fmt) with the same `// Output:`,
  `// ExitCode:` and `// Stderr:` expectations as `TestRun`. `GOCPP_TEST_WINDOWS=all` runs every run test
  except those that `//go:build` leaves out for Windows and those of `windows_skip.txt` (`name: reason`),
  and the log says how many were skipped for each reason. Windows reports a fault as
  `[signal 0xc0000005 ...]`; the harness maps it to Go's SIGSEGV line before it compares.
- `TestWindowsPrograms` runs the programs of `testdata/windows`: what the run tests cannot say (arguments,
  environment, an exit code, UTF-8 output, a stack overflow, big output, a C `exit`).
- The tests of package os (step 14.3): `windows_sample.txt` holds 43 `os_*` run tests whose Linux expectation holds on
  Windows, and the 61 `os_win_*` run tests carry a `//go:build windows` line, so `TestRun` skips them and `TestRunWindows` runs
  them: their `// Output:` is what **Go 1.25.14 built for Windows** (`GOOS=windows go build`, the dev shell's Go) prints when
  it runs on the real PC, checked against gocpp's output there (a sweep of `os_*` in both, on the PC, gave the same output
  for 101 of 114; the others use Linux-only API). `windows_skip.txt` says, for each other `os_*` test, which `os_win_*` twin
  replaces it or why it cannot run. `TestWindowsConsole` runs `testdata/windows/console.gp` in a real console (a pseudo
  console over `ssh -tt`, `internal/winssh.RunConsole`: needs `GOCPP_WINDOWS_SSH`, skips under Wine) and compares what the
  screen shows: UTF-8 through `fmt` and `os.Stdout`, stderr, and a read of the console.
- Windows interop (step 14.4, ADR-082): `testdata/windows` also holds `wstr.gp` (`C.wstr` and `C.GoStringW` with real
  `W` functions), `winapi.gp` (kernel32, advapi32 and user32 through the C API, a `WINAPI` thread callback, the registry,
  LLP64 sizes), `winzoo.gp` (13 Windows headers), `winmacros.gp` (names that are macros of `<windows.h>`) and `winstruct/` (a
  package with C and C++ files that needs the Win32 defaults); their expectations were checked on the real PC. The
  programs `lib*.gp` use libraries that MinGW does not ship: they skip unless `GOCPP_WINDOWS_LIBS=DIR` names a folder
  with `include/` and `lib/` of sqlite3, zlib, SDL2 and raylib for MinGW-w64, which `tools/winlibs.sh DIR` builds in the dev
  shell (network once). sqlite3 and zlib run; SDL2 and raylib are only linked (`buildOnly`: they need their DLL and a display).
- `TestWineIsolation` is the canary: it runs `hello.exe` in a fresh Wine environment and checks that
  `~/.wine`, `~/.local/share/applications`, `~/.config/menus` and `~/.cache/wine` are as they were, that
  the prefix is in the test's folder, and that no process of the environment survives `Close`.

Wine gets a `WINEPREFIX`, `HOME` and `XDG_*` folders of its own (package `internal/wine`), `DISPLAY` and
`WAYLAND_DISPLAY` are removed, and its server is ended by PID when the last Windows test ends or the
harness is stopped. `make ci-windows` runs them (they are not in `make ci`: they skip unless `GOCPP_TEST_TARGET=windows` is set); the
dev shell has wine and MinGW-w64, and without them the tests skip with the reason.

## Caching

`go test` caches the results. The harness tells it what the results depend on: the files in
`cmd/`, `internal/`, `runtime/` and `stdlib/` (the gocpp binary is built from them), the test
files, the examples and the variables above (also `DISPLAY` and `WAYLAND_DISPLAY`). It can't
see the compilers, gdb or the libraries: after you change those, run with `-count=1`. (Like any
flag outside go test's short list, `-update` turns the cache off by itself.)

gocpp's own cache is another matter: the gocpp commands of the tests run with `XDG_CACHE_HOME`
in the harness's temporary folder, so they compile the runtime once per compiler and flags
(about a second each) and never write your `~/.cache/gocpp`. The same folder holds the probe
results on disk (`gocpp/probe/`, [interop.md](../docs/spec/interop.md#the-two-compiler-probes)),
which the gocpp commands of a run share; `GOCPP_NOPROBECACHE=1` turns them off. Go's own test
binaries (`go test ./internal/...`) don't use the probe cache on disk unless `GOCPP_PROBE_CACHE`
names a folder. The Go of the diff tests keeps its usual build cache (`GOCACHE`).

## testdata/

[testdata/](testdata) holds the inputs of the harness's own unit tests
([format_test.go](format_test.go), for the readers of the formats in
[format.go](format.go)). It is never run as test programs, and `make lint` doesn't
gofmt-check it.
