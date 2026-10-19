# Contributing to gocpp

This page is a short guide to how work is done in this repo and which rules the code follows.
The details are in the plan: [plans/2026-10-13-gocpp/](../plans/2026-10-13-gocpp/README.md).
If this page and the plan disagree, follow the plan and tell the manager, so this page gets fixed.

## 1. How work is done

Every step of the plan is built by three roles: a **manager**, a **coder** and a **reviewer**.
They are separate agents or people, and nobody reviews their own code. The full process, with
templates for tickets, reports and reviews, is in
[WORKFLOW.md](../plans/2026-10-13-gocpp/WORKFLOW.md).

1. The **manager** picks the next step, writes a job ticket and marks the step `[~]`.
2. The **coder** builds exactly what the ticket asks, with tests, and runs `make ci`. Then the
   coder hands over the change with a short report: *Changed*, *Tested*, *Unsure*,
   *Plan issues*.
3. The **reviewer** reads the ticket, the plan sections and the whole change, and runs the
   tests. The review starts with the verdict, **changes requested** or **clean**
   (`Review of job 01-1.2, round 1: changes requested`). Then it lists the findings, each with
   a severity (`blocker`, `should-fix` or `nit`), a place, a reason and a fix. It ends with
   `Checked:` and what the reviewer read and ran.
4. **Changes requested:** the coder answers every finding (`1 fixed: <how>` or
   `2 disagree: <why>`) and hands over the next round. The reviewer reviews the whole change
   again, not only the fixes.
5. **Clean:** the manager re-runs the tests, marks the step `[x]` and assigns the next job.

A step is **done** only when both are true:
- the last review is clean: no blocker and no should-fix finding is left (nits may stay);
- the tests are green: `make ci` on GCC and Clang, plus the sanitizer jobs that apply
  (ASan and UBSan for runtime code, TSan for concurrency).

The full list is in [WORKFLOW.md](../plans/2026-10-13-gocpp/WORKFLOW.md) §4.

Rules of the loop:
- Change only what the ticket's *Scope* allows.
- If the plan looks wrong, write it in your report. Don't change the design on your own. The
  manager updates the plan.
- Never skip or weaken a failing test.
- Disagreements go to the manager. The manager decides, or asks the user.

The plan files (root [README](../plans/2026-10-13-gocpp/README.md) §9):
- Read the phase's `README.md` before you start a step. It explains the idea, how it works and
  the steps.
- The phase's `STATE.md` is the checklist: `[ ]` not started, `[~]` in progress, `[x]` done.
- `TODO.md` lists the next jobs.
- Decisions made during the work go into the phase `STATE.md` ("Decisions / notes", with a
  date).
- When a decision in an ADR changes, a new ADR in [docs/adr/](adr/README.md) replaces it. An
  accepted ADR is never rewritten.
- The manager keeps these files up to date.

## 2. Where things live

| Path | What is there |
|---|---|
| `cmd/gocpp/` | the `gocpp` command: `build`, `run`, `test`, `check`, `emit`, `fmt`, `lsp`, … |
| `internal/` | the compiler and the language server: `syntax` (the parser fork), `load`, `expand`, `check`, `lower` (Go → a C++ syntax tree), `cppir` (that tree and its printer), `cppgen` (runs them: `EmitProgram`; `EmitPackage`, one unit per package, is for the golden tests), `interop`, `build`, `lsp`, `diag`, and `version` (the version string and the last finished phase, for test gating) |
| `runtime/` | the C++20 runtime: `include/gocpp/*.hpp`, `src/*.cpp`, `src/os/{linux,windows}/` |
| `stdlib/` | the standard packages: `.gp` files and native `.cpp` files |
| `tools/` | helper programs: `portstd` (copies Go's standard library, Phase 09) and the generator for the example outputs |
| `tests/` | the test harness and its folders `run/`, `errors/`, `golden/`, `diff/`, `interop/`, `debug/` (§3, [tests/README.md](../tests/README.md)) |
| `examples/` | the target programs, each tagged with the phase that must make it pass |
| `editors/vscode/` | syntax highlighting and the language server client |
| `docs/` | `spec/`, `adr/`, `guides/` and this file ([index](README.md)) |
| `spikes/` | the experiments behind the plan. They are evidence, not product code, and stay as they are |
| `plans/` | the plan |
| `flake.nix` | the dev shell for NixOS: `nix develop` gives you Go, Clang, GCC, gdb, ninja, … |
| `go.mod` | `module github.com/huuhait/gocpp` |

The full layout is in the root [README](../plans/2026-10-13-gocpp/README.md) §10.

## 3. Tests come first

- **Every behavior change comes with a test**, in the same change. Write the test first when
  you can: it fails before your change and passes after it. A bug fix comes with a test that
  fails without the fix.
- **Error paths get error tests** (`// ERROR`, see below).
- **Real Go is the reference.** For a program that Go can run, the expected output is generated
  with real `go run`, never written by hand. Go's output is easy to guess wrong.

The test programs in `tests/` and `examples/` run from one Go test binary
(`go test ./tests/...`). `make test` runs all Go tests (`go test ./...`), so it runs them too.
The runtime's C++ unit tests are separate (§5). The test folders
([Phase 00](../plans/2026-10-13-gocpp/phase-00-foundations/README.md) §2.3):

| Folder | A test is | It passes when |
|---|---|---|
| `tests/run/` | a `.gp` program with a `// Output:` block at the end; optional `// ExitCode: N` and `// Stderr: regexp` | `gocpp run` prints that output, exits with `N` (default 0), and stderr matches |
| `tests/errors/` | a `.gp` file with `// ERROR "regexp"` on each line that must fail | `gocpp check` reports exactly those errors, on those lines |
| `tests/golden/` | a `.gp` file and a `.cpp.golden` snapshot of its generated C++ | the generated C++ matches the snapshot; `-update` rewrites the snapshots |
| `tests/diff/` | a `.gp` program that uses only the Go-compatible part of gocpp | real Go and `gocpp run` print the same stdout and exit with the same code. A `// diff: stderr` line also compares stderr, and `// diff: combined` compares both streams merged into one |
| `tests/interop/` | a run test for a program that uses C/C++ headers or libraries | like `run/`; skipped with a message when `pkg-config --exists` doesn't find a library of its `// pkg-config: …` line |
| `tests/debug/` | a `.gp` program with `// gdb:` comments at the end of lines: `print x = 12`, `bt …`, `next N …` | its DWARF line table maps its code to the right `.gp` lines, and gdb (batch mode) stops at those lines and shows those values; the gdb part is skipped when gdb is missing, except in `make ci` (`GOCPP_REQUIRE_GDB=1`) |
| `examples/` | a target program: a folder with `main.gp` and `output.txt` | `gocpp run . <args>`, run in that folder, prints `output.txt` and exits with the expected code |

Keep the golden set small, and change a snapshot only on purpose.

The test comments follow Go where Go has a format: `// Output:` comes from Go examples, and
`// ERROR` from Go's own `test/` folder.
- `// phase: NN` is the first line of **every** test file: each `.gp` test in `tests/` and the
  `main.gp` of each example. It is the phase that must make the test pass, with two digits.
  Until that phase is finished, the harness skips the test with "needs Phase NN".
- `// Output:` starts the expected standard output, at the end of the file. Each output line
  is one `//` comment line below it.
- `// ExitCode: N` is the expected exit code. A panic exits with 2, like Go.
- `// Stderr: regexp` (run and interop tests): standard error must match this regexp. Without
  a `// Stderr:` line, standard error must be empty.
- `// ERROR "regexp"` goes at the end of a line that must fail. The regexp must match the
  error message for that line. `// WARNING "regexp"` works the same way for a warning
  (`file.gp:LINE:COL: warning: message`).

Run and interop tests compare the output like a Go example: space at the start and at the end
of the whole output doesn't count. Examples, diff tests and golden tests compare byte for byte:
an example's output with its `output.txt`, a diff test's output with real Go's, and a golden
test's C++ with its snapshot. The full formats are in [tests/README.md](../tests/README.md)
(and the README in each test folder) and in [examples/README.md](../examples/README.md).

The run, interop, diff, golden, debug and example tests run once with `g++` and once with
`clang++` (`GOCPP_CXX=clang++` tests with clang++ only). Outside CI, a missing compiler is a skip.
`make ci` sets `GOCPP_REQUIRE_CXX=g++,clang++`, so there it is a failure. Tests of phases
after the last built one (`Phase` in `internal/version`) are skipped unless `GOCPP_TEST_PHASE=NN`
runs the tests up to phase NN: `make test`, `make ci`, `make quick` and `make ci-sanitize` set it
to `CI_TEST_PHASE` of the Makefile (10: the features of Phases 06, 08 and 10 that are built have tests),
and a test that still waits for a step of its phase has a `// pending:` line and is skipped
(`GOCPP_TEST_PENDING=1` runs it). While you build phase NN, `GOCPP_TEST_PHASE=NN` runs the tests up to
that phase. `GOCPP_FLAGS` passes more flags to
`gocpp run`, for example `GOCPP_FLAGS=-asan` (ASan and UBSan), which the sanitizer job
`make ci-sanitize` uses ([tests/README.md](../tests/README.md#sanitizers): a test that leaks a
reference cycle on purpose has a `// lsan: cycle` line). `make test-programs` runs only the
test programs and says why each skipped test is skipped.

The emitter's own tests (`go test ./internal/cppgen`) add the flags of `GOCPP_TEST_CXXFLAGS`
to each C++ compile, for example `GOCPP_TEST_CXXFLAGS="-O2 -fsanitize=address,undefined
-fno-sanitize-recover=undefined"`. Their random evaluation-order test runs only with
`GOCPP_RANDOM_TESTS=1`; `GOCPP_RANDOM_N` is the number of programs (default 40),
`GOCPP_RANDOM_SEED` the first seed (a failure names its seed), and `GOCPP_RANDOM_DIR` a folder
that keeps the programs.

A run test (from Phase 00 §2.3; the output comes from real `go run`):

```go
// phase: 03

// tests/run/slices_append.gp
package main

import "fmt"

func main() {
	s := []int{1, 2}
	t := append(s, 3)
	s[0] = 9
	fmt.Println(s, t, len(t), cap(t) >= 3)
}

// Output:
// [9 2] [1 2 3] 3 true
```

An error test (the message is the one `go/types` gives):

```go
// phase: 01

// tests/errors/undefined_name.gp
package main

import "fmt"

func main() {
	fmt.Println(count) // ERROR "undefined: count"
}
```

To get the expected output from real Go, copy the program to a `.go` file and run it:

```sh
mkdir -p /tmp/gp-expect
cp tests/run/slices_append.gp /tmp/gp-expect/main.go
go run /tmp/gp-expect/main.go
```

For `// ExitCode:` and `// Stderr:`, build the program and run the binary. (When the program
fails, `go run` adds its own `exit status N` line to stderr and exits with 1.)

```sh
go build -o /tmp/gp-expect/prog /tmp/gp-expect/main.go
/tmp/gp-expect/prog
echo $?    # in fish: echo $status
```

## 4. Go style

The compiler, the language server and the tools are written in Go 1.25.

- **gofmt.** Every Go file outside `spikes/` and `testdata/` folders is formatted with `gofmt`.
  gopls does it when you save.
- **go vet.** `go vet ./...` reports no problems.
- **staticcheck** finds more problems than `go vet`. Run `make staticcheck` when you change Go
  code.
- **The parser fork.** `internal/syntax/` is a copy of Go's `go/scanner` and `go/parser`
  (from Phase 10 also `go/printer`) with a few small patches. Change only what a patch needs,
  and list every patch in `internal/syntax/FORK.md`, so the fork can move to a new Go version
  ([Phase 01](../plans/2026-10-13-gocpp/phase-01-frontend/README.md) §2.2).
- **The copied LSP code.** `internal/lsp/protocol/` holds gopls's generated LSP types and their
  generator, copied as they are. Don't edit the copied files: `update.sh` copies them again, and
  [`internal/lsp/protocol/COPIED.md`](../internal/lsp/protocol/COPIED.md) lists them and says
  how to update. gocpp's own code there is in its own files.
- **Messages.** Error messages read like Go's own.
- **Comments** explain what isn't obvious.

The make targets for these checks:

| Command | What it runs |
|---|---|
| `make lint` | the gofmt check, which skips `spikes/` and every `testdata/` folder, then `go vet ./...` |
| `make staticcheck` | `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...`, so there is nothing to install |
| `make quick` | the edit-test loop, in a minute or two: only what differs from the merge base with `dev` (gofmt, vet and the tests of the changed Go packages; the changed test fixtures with one compiler, `GOCPP_CXX` or g++; a small smoke set of fixtures; and for `runtime/` changes its fast tests and the C++ unit tests in the debug mode). It is **not a gate**: run `make ci`, `make staticcheck` and, for runtime or emitter changes, `make ci-sanitize` before a change is handed over or merged. `GOCPP_TEST_PHASE=NN make quick` includes the fixtures of the phase being built (it runs the phases up to `CI_TEST_PHASE` by default); `QUICKFLAGS=-n` prints the commands only |
| `make ci` | `lint`, `test` and `build`: every required check, the one that the loop uses. The test programs run with both g++ and clang++, and here a missing compiler fails (§3). It does not run staticcheck, because `go run` downloads staticcheck the first time |
| `make ci-sanitize` | the sanitizer job: the test programs that run code (`tests/run`, `tests/diff`, `tests/interop`, `examples/`) built with `-asan` (ASan, LSan and UBSan), and the emitter's tests (`internal/cppgen`) with the same sanitizers, on g++ and clang++. The repo has no CI service yet, so this target is the CI job: run it next to `make ci` when a change touches the runtime, the emitter or the tests. It runs the tests up to `CI_TEST_PHASE`; `GOCPP_TEST_PHASE=NN make ci-sanitize` runs the tests of the phase being built |
| `make sanitize-nightly` | the nightly sanitizer job (step 15.11, [guide](guides/sanitizers.md)): all of `tests/run` with `-asan` (ASan, LSan, UBSan), UBSan alone and `-race`, with both compilers, the canaries (programs that are wrong on purpose must be reported) and the leak audit of `tools/sanitize/leaks.txt`; any sanitizer report fails it. Hours of CPU time, so it is not part of `make ci`: run it nightly (`tools/sanitize/nightly.sh`) or before a release. `make sanitize-smoke` runs it on 14 programs in two minutes, `make sanitize-leaks` is the leak audit alone, `make sanitize-selftest` tests the job with real sanitizers. A test with `// lsan: cycle` needs a line in `tools/sanitize/leaks.txt` |
| `make vscode-test` | the tests of the VS Code extension in `editors/vscode`: its grammar and its code, including a run of `gocpp lsp` (`npm test`, with `npm ci` first when needed). Not part of `make ci`, because npm downloads the packages the first time. Run it when you change `editors/vscode` or the start of `gocpp lsp` |

`v0.7.0` is staticcheck 2026.1, the newest release that works with Go 1.25. The newer
releases (`v0.8.x`) need Go 1.26.

## 5. C++ style (the runtime)

- **Format with [`.clang-format`](../.clang-format).** It is for the C++ that people write: the
  runtime (`runtime/`) and the native `.cpp` files in `stdlib/`. The style is LLVM's, set up
  like this:
  - 4-space indents, lines up to 100 columns;
  - `T* p` and `const T& r`: the `*` and `&` stay with the type;
  - opening braces on the same line: `if (x) {`, `struct S {`;
  - `template <...>` on its own line, above the struct or function;
  - a short function stays on one line only inside a struct or class (an empty `{}` body also
    stays on one line, anywhere);
  - `public:` lines up with `struct`, the same way `case` lines up with `switch`;
  - includes keep the order you wrote, because the order can matter.

  ```sh
  clang-format -i <files>                  # format the files in place
  clang-format --dry-run --Werror <files>  # only check: fails if a file needs changes
  ```

  Use the clang-format from the dev shell (`nix develop`, version 21). Versions 15 to 20 can
  read the file too (for example an older clangd in your editor), but they can format some
  lines differently.
- **Don't run clang-format on generated C++.** gocpp's emitter decides how its output looks
  ([Phase 02](../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md) §2.2), and
  `tests/golden/` holds snapshots of that output.
- **C++20** (`-std=c++20`, not `gnu++20`). The code must compile with both GCC (13 or newer)
  and Clang (17 or newer) (ADR-003), and every test runs on both.
- **Names.** The runtime's names are in `namespace gp` (`gp::string`, `gp::slice`, `gp::ptr`)
  (ADR-009). The helper names that gocpp generates (temporaries, init functions, field tables)
  start with `gp_`: `gp_t0`, `gp_init_all`, `gp_fields`. User names that start with `gp_` are
  escaped, so the two never collide. Each gocpp package becomes one flat namespace,
  `gpkg_<escaped import path>` (`gpkg_main`, `gpkg_fmt`) (ADR-010).
- **No plain-named members.** Runtime types (`gp::ptr`, `gp::string`, `gp::slice`, `gp::map`,
  …) have only operators and `gp_`-prefixed members. Everything else is a free function, like
  `gp::len(s)`. So a Go method named `get` or `len` can never clash with a runtime member
  ([Phase 03](../plans/2026-10-13-gocpp/phase-03-core-language/README.md) §2.1).
- **The operating system only through `runtime/src/os/`.** In the runtime, nothing outside
  that folder calls the operating system directly (§6).
- **Tests.** The runtime has its own C++ unit tests, built with both compilers
  ([Phase 02](../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md) step 2.7).
  Runtime changes are checked with ASan and UBSan, and concurrency code with TSan.
- **Comments** explain what isn't obvious.

## 6. Platforms: Linux and Windows

- gocpp targets **Linux and Windows**, x86-64 first (ADR-021). Linux comes first. Windows test
  builds start in Phase 06, and the full Windows port is Phase 14. Both must work before v1.0.
  Windows uses MinGW-w64 (clang or gcc) first; the MSVC ABI (`clang-cl`) comes later. macOS is
  not a target.
- **In the runtime, nothing outside `runtime/src/os/` calls the operating system directly.**
  That folder also holds the Phase 08 thread layer `gp::os`. It gets a `linux/` version in
  Phase 02 and a `windows/` version in Phase 14. Everything else in the runtime uses standard C++
  (`std::chrono`, `<atomic>`, `std::filesystem`). A CI check fails if a POSIX header appears
  outside that folder
  ([Phase 02](../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md) §2.6).
- **The standard library's own per-OS files are the other allowed place.** Like Go, the
  standard library keeps its system code in per-platform files (`file_unix`, `file_windows`).
  On Windows, a native `internal/syscall/windows` package calls `<windows.h>`
  ([Phase 09](../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md) §2.1,
  [Phase 14](../plans/2026-10-13-gocpp/phase-14-windows/README.md) §2.3).

## 7. Commit messages

- **One commit per clean job.** The manager makes it, once the user has said that commits are
  OK. Until then, nothing is committed. Coders and reviewers don't commit.
- The message is `phase NN, step N.M: <title>`, for example
  `phase 01, step 1.2: parser patch 1 (keywords after .)`. A job split from a step uses the
  job name: `phase 01, step 1.8a: <title>`
  ([WORKFLOW.md](../plans/2026-10-13-gocpp/WORKFLOW.md) §7).
