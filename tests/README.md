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
| `TestTypedCheckCorpus` | the programs of `interop/`, `zoo/`, `run/`, `pkgs/` and `examples/` that import a header | the same programs as `TestCorpusTyped`, checked only | `gocpp check -interop=opaque` passes (the premise) and `gocpp check -interop=typed` passes too, with g++ and with clang++ (no typed error on a program that compiles in opaque mode: the exit criterion of Phase 12, step 12.12); with clang++ it prints nothing. About 290 programs in half a minute. The tests that expect a failed build (`ExitCode: 1` and a `// Stderr:` pattern with `.gp:`) and the programs of `typedExceptions` are left out |
| `TestDocSnippets` | [docs/spec/interop.md](../docs/spec/interop.md), [interop-cookbook.md](../docs/spec/interop-cookbook.md) | every `<!-- test: PATH -->` block of the two docs | the fenced block after the marker equals the file (without its `// phase:` header lines and `// Output:` block; `<!-- output: PATH -->` shows the output), every `cookbook_*` and `doc_*` interop test is shown in the docs, an output marker has its test marker before it, a shown program that prints has its output shown, every ```go block carries a test marker or `<!-- illustration: reason -->`, and every relative link of the two docs leads to a file and a heading that exist |
| `TestStackOverflow` | [overflow/](overflow/README.md) | a run test of a program that recurses without end (main goroutine, other goroutine) | exit code 2 and `fatal error: stack overflow` with a `goroutine N [running]:` line on stderr, as in Go, never a bare SIGSEGV; skipped under `-asan` and `-race`, where the sanitizer reports the overflow |
| `TestZoo` | [zoo/](zoo/README.md) | the header zoo: one `.gp` program per header or library (the C and C++ standard headers, sqlite3, zlib, SDL2, raylib), with an `// Output:` block | `gocpp check` is clean, and `gocpp run` prints that output with each compiler. A file with `// display: required` (SDL2, raylib) is only built, never run |
| `TestPkgs` | [pkgs/](pkgs/README.md) | a folder `pkgs/<name>/` with a `gocpp.mod`, a `main.gp` (with `// Output:`) and the packages it imports | `gocpp run .` prints that output, like `TestRun`; a program of several packages (Phase 06) |
| `TestPkgsVsGo` | [pkgs/](pkgs/README.md) | the same folders, with `GOCPP_RUN_VS_GO=1` | real Go prints that output (the module is copied with `.gp` renamed to `.go`) |
| `TestPkgErrors` | [pkgs/errors/](pkgs/README.md#errors) | a module folder with `// ERROR` comments in any of its `.gp` files | `gocpp check -e ./...` reports exactly those errors, on those lines |
| `TestDebug` | [debug/](debug/README.md) | a `.gp` program with `// gdb:` comments (stops with `print`, `bt` and `next`) | its binary's DWARF line table maps the program's code to the right `.gp` lines (and nothing else to the `.gp` file), and a gdb session in batch mode stops at the `.gp` lines and shows what the comments say; the gdb part is skipped when gdb is missing or may not trace programs (a failure with `GOCPP_REQUIRE_GDB=1`, as in `make ci`) |
| `TestClangdOpensGeneratedCode` | 13 programs from `examples/` and `tests/` (a hello, modules of several packages, generics, goroutines and channels, interfaces, macros, `!`, `#Derive`, standard-library packages written in gocpp, C and C++ files in the module, a C header, sqlite3) | `gocpp export compdb` (the file that a debug build writes) and then `clangd --check=FILE` (batch mode, no editor) over the generated `.gp.cpp`, `pkg.hpp`, `pkg_desc.cpp` and `pkg_init.cpp` files of the program's own packages (and the module's `.c`/`.cpp` files), for each compiler: clangd used the compile command of the compilation database (a header: one inferred from a neighbor) and the command names that compiler; clangd read no configuration file (the runs get an empty `XDG_CONFIG_HOME`, because a user's `config.yaml` can suppress every diagnostic); no warning; no error line except the refusals of its refactoring tweaks; no `file not found`, `unknown type name` or `Unable to handle compilation`; the line `All checks completed, N errors` has `N` equal to those refusals (clangd's own exit code is not usable: it counts the refusals as errors). One known clangd 21 bug is accepted in headers only and logged: `Failed to resolve URI` from its DefineOutline tweak on a header with a method that has an inline body next to a member that is only declared (a plain C++ header does it too; clangd does not count it). Skipped, with the reason, when clangd 21 is not on PATH; a failure with `GOCPP_REQUIRE_CLANGD=1` (`make ci`). `TestClangdVerdict` tests the reading of the log, `TestClangdChecksCatchErrors` shows that a bad file next to the generated C++ (an undeclared name, a missing header, an unknown type) is found, and `TestClangdIgnoresDeveloperConfig` that a `config.yaml` with `Diagnostics: {Suppress: "*"}` in the user's `XDG_CONFIG_HOME` does not hide it |
| `TestClangdGoToDefinition` | `tests/pkgs/basic_cross` | the language server of clangd, started as an editor does, lists `main` in the symbols of the generated `main.gp.cpp` and finds the definition of a function of another package that `main` calls, in that package's generated files; the gate is the one above |
| `TestClangdLineDirectivesPointAtGP` | the programs above | every `#line` directive of the generated sources names a `.gp` file of the module (or of the embedded standard library) that exists, at a line inside it, or the generated file itself; needs no clangd. (clangd ignores `#line` for its diagnostics: it reports lines of the generated file) |
| `TestGDBPrinters` | [gdb/](gdb/README.md) | a program `gdb/NAME.gp` with `// @tag` lines, and a session `NAME.gdb` (gdb commands) with the output that gdb must print (`NAME.golden`, or `#want` regexps) | gdb's output for each command, with the printers that every program carries (`runtime/debug/gocpp-gdb.py`): Go values, Go names in backtraces, the goroutine list; built at `-O0` and `-O2`, with trimmed paths, and with `-asan`; skipped like `TestDebug` |
| `TestLLDBPrinters` | [gdb/](gdb/README.md) | a session `gdb/NAME.lldb` for the program `gdb/NAME.gp` | the same for lldb and `runtime/debug/gocpp_lldb.py` (best effort); skipped when lldb is missing |
| `TestPhase10NoHelpers` | the Phase 10 programs of [run/](run/README.md) (`must_*`, `umacro_*`, `macro_*`, `derive_*`), the module folders of [pkgs/](pkgs/README.md) with these names and the golden tests `must`, `macros_user` and `derive` | the C++ that `gocpp emit` prints names no helper of the hidden packages `gocpp/must` and `gocpp/macro` (`!` is lowered to plain control flow, macros and `#Derive` are plain Go before the lowering: the exit criterion "no helper calls left in generated C++"); `TestHelperMarkers` tests what counts as a helper |
| `TestPhase10ExamplesBothCompilers` | `examples/must-errors` and `examples/macros` | each runs with g++ and with clang++ and prints `output.txt`, and the example's `twin/` built and run by real Go prints the same bytes (the exit criterion "`must-errors` and `macros` examples pass on GCC and Clang") |
| `TestPhase10ExpansionIsInline` | the programs of `tests/testdata/phase10` (macros and `!` across packages), `bench/phase10` and three `pkgs/` modules | the C++ of the program's own units has no hidden helper, no `gp::func`/`std::function`/allocation in the parameters and arguments of a macro instance, a plain `if (must_eN != nullptr)` after each `!`, and a body for every macro instance (so that the C++ compiler inlines it across packages); `TestInlineProblemsFindWrappers` tests the checks |
| `TestPhase10BangEmitsHandWrittenCode` | `bench/phase10/must_call` and `tests/testdata/phase10/bang_forms` | the C++ of the program with `!` equals, up to the lowering's names and the block around a `!` statement, the C++ of the program with the error tests written by hand |
| `TestPhase10MachineCodeEquivalence` | a wrap macro, a must call and a wrap macro with a must call of another package (`bench/phase10/wrap`, `bench/phase10/must_call`, `tests/testdata/phase10/cross_wrap`) | g++ and clang++ at `-release` make the same machine code (`nm`, `objdump`: addresses and rip displacements normalized, symbols, callees and jump offsets kept) for the program with the feature and the hand-written one, in every function of the program: "no measurable overhead" without a clock (Linux; the timings are in [docs/benchmarks/must-macros.md](../docs/benchmarks/must-macros.md)) |
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

The syntax and the meaning of `!` (must), macros (`macro`, `lazy`, `#Name(...)`, `#file()` and the other
built-ins), `#Derive` and `//go:embed` have no phase gate (step 10.12,
[ADR-095](../docs/adr/095-must-and-macros-are-always-available.md)), so a test of them does not say
`// phase: 10`. Its line names the latest gate that its program meets: `// phase: 05` for a program that uses only
what the finished phases have (it runs in a plain `go test ./tests`, with no `GOCPP_GATE_PHASE`), `// phase: 08` for one
that starts goroutines or uses channels, `// phase: 09` for one that imports `strings`, `os`, `strconv` and the other
packages of Phase 09. To find the line of a new test, run `gocpp check -e` on it at each phase
(`GOCPP_GATE_PHASE=NN`): the line is the latest phase named in a "not supported yet (Phase NN)" or "not available yet
(Phase NN)" error. `// phase: 10` stays for a test that needs something else of Phase 10 (`errors.Trace`).

A test is also skipped, with a message, when:
- it has a `// zoneinfo: required` line (run tests: a program that loads time zones by name with `time.LoadLocation`) and the machine has no time zone database with `America/New_York`, `Europe/Berlin` and `Asia/Kolkata` (the `ZONEINFO` variable, else `/usr/share/zoneinfo`, `/usr/share/lib/zoneinfo`, `/usr/lib/locale/TZ`, `/etc/zoneinfo`);
- it has a `// pending: reason` line (below the phase line; allowed in run, diff, interop and pkgs tests and in
  examples) and `GOCPP_TEST_PENDING` is not `1`. It says that the test's phase is only partly built and
  the test fails until a named step lands: `make ci` runs the tests up to Phase 10 (`GOCPP_TEST_PHASE=10`,
  below), and the example `macros` is pending. The step that makes the test pass removes
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
module next to the test (`difftest/internal/<name>`, [twin_test.go](twin_test.go); `testing/internal/testdeps` is `difftest/internal/testing_testdeps`), and builds the program there. The
natives of `internal/bytealg` (C++ in gocpp) are replaced in that copy by calls of `bytes` and `strings`
(`testdata/twin/bytealg_native.go.txt`), so a test's `// Output:` block must equal what Go computes for the same inputs.

## Go's own tests of the standard packages

`tests/stdgo` holds Go 1.25.14's `_test.go` files for the tier-1 packages (and `encoding/json`), converted to `.gp` and patched, and one manifest for each package that says what happened to every test function
(step 9.10, [ADR-113](../docs/adr/113-go-s-tests-of-the-standard-packages-run-in-the-shipped-package-and-stdgocount-counts-them.md), [docs/guides/stdlib-tests.md](../docs/guides/stdlib-tests.md)). They run in the packages that
gocpp ships: `GOCPP_TEST_STDLIB=$PWD/tests/stdgo gocpp test strings`. `go run ./tools/stdgocount -detail strings` builds, runs and counts them by the rule of the Phase 09 README 2.8 (`-check` fails on an
unexplained failure or a package under 90 %); it takes minutes for the large packages, so it is not part of `go test ./...`. `go test ./tests/stdgo` checks the manifests, that the committed files are what
`tools/portstd/gotests/convert_std.py` makes, and runs the tests of `cmp` and `path` (skipped with `-short`). The range-over-func torture test of `test/rangegen.go` runs with
`python3 tools/portstd/gotests/rangegen_shards.py -gocpp bin/gocpp` (800 units in 32 shards).

## Stress tests

The programs `tests/run/stress_*.gp` (step 8.13: goroutines, channel operations, recursion, timers, mutexes and
`select` under contention) read `GOCPP_STRESS` with `os.Getenv`. Without it they run a **small** size that finishes
in a few seconds (500 goroutines, 10 thousand channel operations), so they are ordinary members of `TestRun`, of the
sanitizer runs and of `TestRunVsGo`. With `GOCPP_STRESS=1` they run the **big** size (10,000 goroutines alive at
once, about 4 million channel operations, 20,000 timers, 64 goroutines x 20,000 lock rounds, recursion 50,000 frames
deep), and each program checks its own totals and prints only `true` lines, so one `// Output:` block serves both sizes:

```sh
GOCPP_STRESS=1 GOCPP_TEST_PHASE=09 go test ./tests -run 'Run/stress_'                         # all five, big
GOCPP_STRESS=1 GOCPP_TEST_PHASE=09 go test ./tests -run 'Run/stress_goroutines'                # one, by name
GOCPP_STRESS=1 GOCPP_RUN_VS_GO=1 GOCPP_TEST_PHASE=09 go test ./tests -run 'RunVsGo/stress_'    # the same with real Go
```

Go's own `test/chan` programs run through the conformance tool, not here: `go run ./tools/gotest run -filter '^chan/' -v`
(add `-cxx clang++` for the other compiler; [docs/guides/conformance.md](../docs/guides/conformance.md)). 18 of the 19 pass (with g++; `sieve2` since step 9.11, when `container/heap` and `container/ring` came) and are in the
regression gate; the other one, `select2`, reads the runtime's memory statistics and is skipped as `runtime-internals`.

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
Today 23 tests have it (`go run ./tools/sanitize leaks -check` compares them with the list below; `rg -l "^// lsan: cycle" tests internal` also finds the three header-parser fixtures of `tests/testdata/header`); the first ones were `tests/run/closures_recursive.gp` and
`internal/cppgen/testdata/emit/closures.gp`, both a closure that calls itself through a variable,
`tests/run/embed_later_types.gp`, two structs that embed pointers to each other, and
`tests/run/sync_cond_queue.gp` and `sync_cond_semaphore.gp`, a struct that holds a `sync.Cond` whose
Locker is a pointer to a field of that same struct (the usual `sync.NewCond(&q.mu)`); the others are later tests of the same kinds, and `derive_cycle.gp` (a value that points to itself).

**The marker hides every leak of its program, so it is bounded** (step 15.11,
[docs/guides/sanitizers.md](../docs/guides/sanitizers.md)): every test with `// lsan: cycle` must be listed in
`tools/sanitize/leaks.txt` with the kind of cycle, the reason and the most it may leak (a ceiling in bytes and
allocations per compiler). `go test ./tools/sanitize` (part of `make test`) fails when a marker and a line
disagree, and the leak audit (`make sanitize-leaks`, a stage of the nightly job) builds each listed test with
LeakSanitizer on and fails when it leaks more than its ceiling, leaks nothing any more (remove the marker) or
makes any other sanitizer report. Adding the marker to a test means adding its line (`go run ./tools/sanitize
leaks -update -filter NAME` measures the ceiling).

## The nightly sanitizer job

`make sanitize-nightly` (step 15.11, [docs/guides/sanitizers.md](../docs/guides/sanitizers.md)) runs every
program of `tests/run` with `-asan` (AddressSanitizer, LeakSanitizer, UBSan), with UBSan alone and, for the
concurrency programs and the silent programs of `tests/race`, with `-race`, with both compilers, and fails on
*any* sanitizer report in a program's stderr, whatever the test expects; it also runs the canaries (programs
that are wrong on purpose, which must be reported) and the leak audit. It reads the same header lines as
the harness (`// phase:`, `// pending:`, `// ExitCode:`, `// lsan: cycle`, `//go:build`), skips the tests that
need C libraries or a display, and does not read `// Output:` (the harness checks that). `make
sanitize-smoke` runs it on 14 programs in about two minutes, `make sanitize-selftest` proves with real
sanitizers that it fails on an injected UBSan, ASan, LeakSanitizer and ThreadSanitizer error. The interop, diff,
zoo and example programs and the emitter's tests stay with `make ci-sanitize` and `make ci-race`, which the
nightly job runs as its `deep` stages (`deep-sanitize`, `deep-race`).

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

The clangd tests (`clangd_test.go`) need clangd 21 (the dev shell has it; its clangd is wrapped and finds the standard headers). Without it, or with another major version, they skip with a message;
`make ci` sets `GOCPP_REQUIRE_CLANGD=1`, which makes that a failure. clangd runs under a time limit in a process group of its own that is killed when it ends.

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
| `GOCPP_REQUIRE_CLANGD` | none (`1` in `make ci`) | `1`: the clangd tests (`TestClangd*`) fail instead of skipping when clangd is not on PATH |
| `GOCPP_FLAGS` | none | more flags for `gocpp run`, split on spaces, for example `GOCPP_FLAGS=-asan` for a sanitizer run |
| `GOCPP_TEST_PHASE` | `Phase` of internal/version | run the tests up to this phase (00 to 15), for example `GOCPP_TEST_PHASE=01` while you build Phase 01 |
| `GOCPP_GATE_PHASE` | set per test | not set by you: for gocpp's own development and tests, gocpp's phase gates accept the features up to this phase (`cmd/gocpp/gatephase.go`). For a test whose `// phase:` comes after `Phase` of internal/version (one that runs only with `GOCPP_TEST_PHASE`), the harness passes that test's phase to its gocpp commands, so the test can use the features of its phase before the phase is finished. A test of a finished phase gets none, so an error test still sees the gates of later phases. It never turns the gates of finished phases back on |
| `GOCPP_WORKLOST` | `fail`, set by the harness | not set by you: the gocpp commands of the tests stop with status 4 when something else removes a build's work folder, instead of building again as gocpp does for users ([docs/spec/build.md](../docs/spec/build.md#a-work-folder-that-is-removed)), so a real bug is not hidden. `cmd/gocpp/worklost_test.go` tests both |
| `GOCPP_RUN_VS_GO` | none | `1`: `TestRunVsGo` builds each test in `tests/run` with real Go (`go build`, like a diff test) and checks the test's `// Output:` block (compared like `TestRun` compares), `// ExitCode:` and `// Stderr:` regexps against what the Go binary does, so that an expected output pasted from Go can't drift. It checks the tests of every phase, runs no C++ compiler, and skips a test whose code `go build` rejects (an error at a line of the program: gocpp-only code) and a test with a `// divergence: Dn` line (it shows a divergence of [docs/spec/language.md](../docs/spec/language.md), so Go does something else on purpose). Any other `go build` failure fails the test, and a run that compares no test fails too. Off, `TestRunVsGo` is a skip: `GOCPP_RUN_VS_GO=1 go test ./tests/... -run RunVsGo` |
| `GOCPP_TEST_STDLIB` | none | a folder with one subfolder for each import path: its `_test.gp` files and `testdata` join the embedded standard package, so `gocpp test strings` runs Go's own tests (`tests/stdgo`, [docs/guides/stdlib-tests.md](../docs/guides/stdlib-tests.md)); `tools/stdgocount` sets it |
| `GOCPP_TEST_ALL_ERRORS` | none | `1`: `gocpp check` and `gocpp test` print every error of a package, not the first ten |
| `GOCPP_TEST_STDGO_OUT` | `tests/stdgo` | the folder `convert_std.py` writes to (`tests/stdgo/stdgo_test.go` makes the tests in a temporary folder to compare them with the committed ones) |
| `GOCPP_TEST_PENDING` | none | `1`: also run the tests with a `// pending:` line (they fail until their step lands) |
| `GOCPP_TEST_TIMEOUT` | `2m` | the time limit of one command (one `gocpp run`, one `go build`), as a Go duration |
| `GOCPP_TEST_TARGET` | none | `windows`: also build the run tests for windows/amd64 and run them under Wine (`TestRunWindows`, `TestWindowsPrograms`, `TestWineIsolation`; the section "Windows under Wine" below); run on a real PC with `GOCPP_WINDOWS_SSH` |
| `GOCPP_TEST_WINDOWS` | `sample` | with `GOCPP_TEST_TARGET=windows`: `sample` runs the tests of `windows_sample.txt`, `all` every run test that `windows_skip.txt` does not name |
| `GOCPP_WINDOWS_LIBS` | none | with `GOCPP_TEST_TARGET=windows`: a folder with `include/` and `lib/` of sqlite3, zlib, SDL2 and raylib built for MinGW-w64 (`tools/winlibs.sh DIR`); `TestWindowsPrograms/lib*` skip without it |
| `GOCPP_WINDOWS_SSH` | none | `user@host`: the Windows tests (`GOCPP_TEST_TARGET=windows`) run their programs on that real Windows PC over ssh instead of under Wine (key authentication; `internal/winssh`; the variable is the only place that names the machine; never commit it) |
| `GOCPP_WINE_JOBS` | `4` | the programs that run under Wine at once (Wine's server is one process) |
| `GOCPP_REQUIRE_WINE_OVERFLOW` | none | `1`: `TestWindowsPrograms/overflow` fails instead of skipping when this machine's Wine ends a stack overflow with exit code 1 and no message |
| `GOCPP_TEST_DOWNLOAD` | none | `1`: `TestWindowsNativeDownload` downloads the pinned llvm-mingw (190 MB) on the real PC (`GOCPP_WINDOWS_SSH`), builds with it and removes it |
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

## gocpp.exe on a real Windows PC

With `GOCPP_WINDOWS_SSH=user@host` (a Windows 10/11 PC with a MinGW-w64 in PATH, key authentication; never Wine),
`windows_native_test.go` (step 14.2, [ADR-081](../docs/adr/081-windows-host-build-driver.md)) copies a cross-built
`gocpp.exe` into the run's folder `%USERPROFILE%\gocpp-wintest\<run id>\` and runs it there, natively:
`TestWindowsNative` (the compiler found and the messages when there is none, `-o prog` -> `prog.exe`,
`compile_commands.json`, `gocpp run` and its exit code, CRLF sources, folders with spaces, a module below a path of
over 260 characters, a DLL with its import library and a C caller, a static library, twelve std-only interop programs, the
manifest, commands longer than 32,767 characters, `GOCPP_CXX` and flags with spaces, errors with Windows paths, two builds
at once), `TestWindowsNativeUnits` (the Windows-only tests of `internal/build` (job objects), `internal/filelock`,
`internal/fsretry`, `internal/toolchain`, `internal/modfetch` and `cmd/gocpp`, built here with `go test -c` and run there)
`TestWindowsNativeGoTest` (`gocpp test -v` of a module with passing, failing and skipping tests, `t.TempDir` with a read-only file, `t.Setenv`, `t.Cleanup`, `t.Chdir`, a benchmark with its `cpu:` line and an example, step 14.9) and the opt-in `TestWindowsNativeDownload`.
`TestWindowsCrossGoTest` (`windows_gotest_test.go`) makes the same module's test binary here with `gocpp test -c -os windows` and runs it on the PC. The cache, `TEMP` and `APPDATA` of the runs are inside the run's folder, which is
deleted at the end. Without the variable these tests skip; with it and an unreachable PC they fail.

## The Windows suite

`TestWindowsSuite` (`windows_suite_test.go`, step 14.7, [ADR-111](../docs/adr/111-the-windows-suite-runs-on-the-real-pc-in-batches-with-an-agent.md),
[docs/guides/windows-testing.md](../docs/guides/windows-testing.md)) runs **every** test of `tests/run` and `tests/pkgs` that applies to
`windows/amd64` on the real PC: it builds them here (`gocpp build -os windows`), copies ONE archive for each batch of 150 to the PC
over ssh, runs them there with an agent (`internal/wintest`: 8 at a time, a time limit, a job object that ends the process tree, outputs
to files) and compares the outputs with the same `// Output:`, `// ExitCode:` and `// Stderr:` as `TestRun`. It is not in `make ci`.

```
GOCPP_WINDOWS_SSH=user@host make test-windows-smoke    # the 44 tests of windows_suite_smoke.txt (about 2 minutes)
GOCPP_WINDOWS_SSH=user@host make test-windows          # the whole suite
make test-windows-list                                 # the dry run (no PC needed): what runs, what is skipped and why
make test-windows-failed                               # only the tests that failed or have no result in the results file
```

`GOCPP_WINDOWS_SUITE=1` turns the test on (the make targets set it); `GOCPP_WINDOWS_SSH` names the PC (without it, or
`GOCPP_WINDOWS_WINE=1`, the test fails: Wine is an explicit option, never the default). Selection: `GOCPP_WINDOWS_ONLY` (a regexp on `dir/name`
or `@file`), `GOCPP_WINDOWS_DIRS` (`run,pkgs`; `interop` can be added). Speed: `GOCPP_WINDOWS_JOBS` (programs at a time on the PC, 8),
`GOCPP_WINDOWS_BUILD_JOBS` (12), `GOCPP_WINDOWS_BATCH` (150), `GOCPP_WINDOWS_TIMEOUT` (2m a program). Results:
`GOCPP_WINDOWS_RESULTS` (default `.windows-suite/results.json`, written after every batch) with a sorted summary and a list of the
failures next to it; `GOCPP_WINDOWS_RESUME=1` keeps the passes of the same checkout state, `=failed` across changes (run only what failed or is new). `windows_skip.txt` is read as a list of **expected
failures** (they run, and one that passes is an error: a stale entry; `GOCPP_WINDOWS_SKIPS=ignore` skips them instead). Stop a run with Ctrl-C, never kill `go test` (an orphaned binary ends its run on the PC itself within a second). Two runs never
use the PC together (a lock file, `GOCPP_WINDOWS_LOCK`), and the PC's run folder is removed at the end. The unit tests of the driver run on
Linux with a fake machine (`go test ./internal/wintest/...`); `TestWindowsSuiteAgent` tests the agent on the PC (hung program and its
children, flood, exit code, no leftovers); `windows_suite_pc_test.go` tests, on the PC, the clean-up of old run folders (with decoy
folders that must survive), `KillRun` (only its own run's processes), `Remove`, and an interrupted run (SIGTERM mid-batch leaves nothing on
the PC or here); all need `GOCPP_WINDOWS_SSH`; `TestWindowsSuite{Env,Selection,...}` check the lists without a PC.

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

## Concurrent builds

`tools/buildstress` runs many `gocpp build` and `gocpp run` processes at the same time against one cache
folder (an editor next to a terminal, CI jobs, `make -j`): small programs of their own and one program that
all of them share (so the same folder of generated files), both compilers, the default build and `-release`,
several runtime keys (each a runtime library of its own), a cache that is replaced by an empty one every few
builds (cold: the runtime library, the precompiled header and the objects are made while others ask for
them), a tiny object cache that is trimmed all the time (`-objsize 2M -trim-marker-every 1s`),
`GOCPP_NOOBJCACHE` for some builds, and `taskset`/`nice` to squeeze the processes. It checks the exit status
and the program's output and counts the folders a build leaves behind; nothing is retried. `-wipe-every 2500ms`
adds a cleaner that removes the work folder of a running build, as a cleaner of `/tmp` would (`-wipe-mode
contents|glob|tmp` for the other ways: `glob` is `rm -rf folder/*`, which leaves the dotfiles). Run it in the dev shell, bounded, through the machine's slot script:

    go build -o /tmp/gocpp ./cmd/gocpp
    go run ./tools/buildstress -gocpp /tmp/gocpp -procs 24 -builds 500 -duration 3m -cold-every 40
    go run ./tools/buildstress -gocpp /tmp/gocpp -procs 24 -builds 150 -wipe-every 2500ms

It exits with status 1 when any build failed. Without the cleaner the builds run with `GOCPP_WORKLOST=fail`
(a lost work folder is an error, as in all the tests and campaigns of the repository; with the cleaner the
default self-healing is what is tested).

## testdata/

[testdata/](testdata) holds the inputs of the harness's own unit tests
([format_test.go](format_test.go), for the readers of the formats in
[format.go](format.go)). It is never run as test programs, and `make lint` doesn't
gofmt-check it.
