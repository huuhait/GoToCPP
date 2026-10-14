# Phase 02: Walking Skeleton · STATE

**Status:** `[~]` in progress (steps 2.1–2.9 and 2.11 done)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] 2.1 Emitter skeleton (writer, `#line` with absolute paths repeated where needed, prologue, flat namespace, prototypes, main wrapper)
- [x] 2.2 Names
  - [x] C++ keywords, reserved names, user names starting with `gp_`
  - [x] the runtime's macro names (`EOF`, `errno`, `assert`, `stdin`, …)
  - [x] `gp_` temporaries
  - [x] shadowing renames (`v := v`, `x := x + 1`)
- [x] 2.3 Expressions
  - [x] constants via `go/constant` (all basic types, big values)
  - [x] string literal escaping (octal, never `\x`)
  - [x] operators (incl. `&^`, unary `^`) + **small-integer cast rule** (int8/int16/uint8/uint16)
  - [x] `gp::div/rem/shl/shr/f2i` (return the Go type; `MIN/-1`, `MIN%-1`, per-type float→int)
  - [x] conversions between basic types
  - [x] calls
- [x] 2.4 Statements
  - [x] `var`, `:=`, `=`, parallel assignment, op-assign, `++/--`
  - [x] `if` with init
  - [x] `for` (all forms; hidden counter for `range` over int; body block before labels)
  - [x] `switch` via case index + native switch (`fallthrough`)
  - [x] labeled `break`/`continue`, `goto` (temporaries in blocks)
  - [x] `return` (single and multiple)
- [x] 2.5 Minimal evaluation-order pass (incl. loop-condition / `else if` / `case` shapes)
- [x] 2.6 Functions: params, multiple results (`std::tuple`), prototypes
- [x] 2.7 Runtime v0 (`gp::string` minimal, helpers, panic as exception, print/println, unbuffered-order stdout, fmtshim, run_main, OS layer `runtime/src/os/` + no-POSIX-outside check) + C++ unit tests
- [x] 2.8 Driver: `build`, `run`, `emit`, `-x`, `-work`, `-asan`, compiler detection, embedded runtime
- [x] 2.9 Internal-error path for C++ compile failures
- [~] 2.10 Tests: ≥ 25 run tests, 3 golden, ≥ 25 differential vs Go (promotion, order, shadowing, float format, float→int, stdout/stderr order)
- [x] 2.11 Debug test: gdb batch + line-table check

## Exit criteria

- [ ] `hello`, `fizzbuzz`, `primes` run correctly on GCC and Clang
- [ ] All subset run + differential tests green on both compilers
- [ ] gdb breaks at a `.gp` line and prints a local; line table checked
- [ ] Golden outputs reviewed and readable
- [ ] Unsupported features give phase-gate errors only

## Decisions / notes

- 2026-10-13 (audit): Go-vs-C++ differences found by running the same code in Go and both
  compilers, now handled here: integer promotion, call order (g++ and clang++ differ),
  `v := v` shadowing, `#line` mapping of multi-line expansions, `goto` over initializations,
  per-type float→int, stdout order, macro names from runtime headers. Panics are C++ exceptions
  from this phase on, and namespaces are flat (`gpkg_main`).
- 2026-10-13 (your decision, Q8): Linux and Windows are the targets, so the runtime reaches the
  OS only through `runtime/src/os/` from the start, and `#line` escapes Windows paths.
- 2026-10-13 (dev-shell review): `-U_FORTIFY_SOURCE` doesn't beat Nix's hardening wrapper (it re-adds
  `-D_FORTIFY_SOURCE=3` after user flags), so `rt.hpp` undefines it first thing when not optimizing;
  a driver test simulates a user's hardened shell with `NIX_HARDENING_ENABLE`.
- 2026-10-13 (docs job 0.6b): `-asan` moved here from Phase 06, because the Phase 03 CI job
  (step 3.18) and the harness (`GOCPP_FLAGS=-asan`) need sanitizer builds before Phase 06.
- 2026-10-13 (runtime review 2.7): fmtshim's Printf handles only `%v %d %s %t %f %%` (and Go's error
  forms; int64_t shows as "int" there until Phase 05), so the emitter (2.3/2.8) rejects constant
  formats with flags, width or other verbs with a phase-gate error (Phase 03 adds them).
  stdout writes wait on a non-blocking fd (poll) and a broken pipe raises SIGPIPE, like Go.
- 2026-10-13: Step 2.7 (runtime v0) committed, clean after 4 review rounds. The reviews added
  `TestSymbols` (every strong global symbol of the runtime is in `gp::` or starts with `gp_`, so it
  can't clash with user C/C++ code), the no-POSIX-outside-`src/os/` check, and pipe/SIGPIPE tests.
- 2026-10-13: Job 2.1 (+2.2, 2.6: emitter skeleton, names, functions; `cppgen.EmitPackage`) committed,
  clean after 2 review rounds. Decisions: a call to a package-level function is written
  `gpkg_main::f(...)` when any argument's C++ type is not a number or bool, so C++ argument-dependent
  lookup can't pick a `gp::` or `std::` function (`func len(s string)` broke both compilers);
  `funcCallName` is the one place for this rule. Constants in places where C++ deduces the type
  (runtime template arguments, `auto`) get their exact Go type (`int64_t(5)`). `#line` only where the
  line is not previous + 1 (the §2.1 example shows more; the rule wins). Escaping is one-to-one by
  name "families" (`new` → `new_`, `new_` → `new__`); the integer type names (`int64_t`, …) are
  reserved too. `?` after `?` is written `\?` (trigraph warnings). `Config.OutputPath`: the 2.8 driver
  must set it. The emitter already does package-level variables and `init` (Phase 03 step 3.16, basic
  types only); the gates still say Phase 03, so its tests run with `Config{Phase: check.LastPhase}`.
- 2026-10-13 (emitter review 2.1): `int`/`int64` are both `int64_t` and `uint`/`uint64`/`uintptr` are
  `uint64_t`, so fmtshim prints Go's type name `int`/`uint` where Go says `int64`/`uint64`/`uintptr`
  (`%!s(int64=5)`). The emitter must pass a type tag to fmtshim for these (or it is a documented
  divergence); decide before Phase 03's `%T`/`%v` work (step 2.3 or 3.x).
- 2026-10-13 (manager): the flaky runtime test `TestCXX/*/diff/bigerr` was real Go's output, not
  gocpp's: Go's runtime `println` writes once and drops what a short write leaves, and a `SIGURG`
  preemption signal can cut a blocking pipe write short (all six configurations failed at once,
  because they share one Go run, at exactly 128 KiB). The Go reference runs now use
  `GODEBUG=asyncpreemptoff=1`.
- 2026-10-13: Job 2.3 (expressions, `internal/cppgen/ops.go`) committed, clean after 1 review round. The
  reviewer's random-expression differential test (about 360k expressions over all Phase 02 operators
  and types, both compilers, -O0 to -O2, with UBSan) matched real Go everywhere. Decisions:
  comparisons of small integers get no cast (the operands are already in range); constants are
  written in decimal; extra parentheses where `-Wall` asks for them. Step 2.10 should keep such a
  random-expression test in the repo, opt-in through an environment variable.
- 2026-10-13 (expressions review 2.3): valid Go can make the compilers warn: clang's
  `-Wtautological-compare` (`x == x` on integers) and g++'s `-Woverflow` when it folds part of an
  expression (`65535 * uint16(x | -1)`); the behavior is right (`-fwrapv`). Decision: the driver
  passes `-Wno-tautological-compare` (clang) and `-Wno-overflow` (g++) for generated files, and the
  emitter tests use the same flags; other warnings stay visible (job 2.8).
- 2026-10-13 (statements review 2.4, for step 2.5): `x, y := a/b, f()` with b == 0 panics before calling
  f (Go calls f first), and so does `x = a/b + f()`: `bind`'s direct path for all-new targets writes the
  values in order without temporaries. The 2.5 hoisting must cover `bind`'s direct path and the
  `forInit` path (control.go) too; parallel assignment to existing variables already matches Go.
- 2026-10-13: Job 2.4 (statements: `stmt.go`, `control.go`, `flow.go`) committed, clean after 2 review
  rounds; examples/fizzbuzz and primes emit and match. The reviewer's random-statement differential test
  (~9,900 functions, both compilers, -O0/-O2/UBSan) matched Go. Deviations from README §2.4 (the
  table shows the first design): labels are `gp_L_break`/`gp_L_continue`/`gp_cont_N` (no clash with user
  labels), a break/continue to the innermost statement stays plain; switch has three shapes (an integer
  tag with constant cases → a direct C++ switch; no fallthrough and no plain break to the switch → an
  if-else chain; otherwise the case index + native switch); fewer temporaries (`a, b = b, a` uses one);
  a range over a constant has no `gp_n`; op-assign is `x = x op y` (Phase 03 must evaluate the target
  once). Step 2.10 should keep the random statement test, opt-in, next to the expression one.
- 2026-10-13: Job 2.8 + 2.9 (the driver: `gocpp build`, `run`, `emit`; `internal/build`) committed, clean
  after 3 review rounds. The Phase 02 harness (`GOCPP_TEST_PHASE=02 go test ./tests/... -run
  'Run|Diff|Examples'`) passes on g++ and clang++, also with `-asan`: hello, fizzbuzz, primes,
  small_int_wrap and 3 run tests. Decisions that differ from README §2.7 or add to it:
  - the runtime is extracted to `<user cache>/gocpp/rt/<version>-<hash16>/` (a "dev" build changes the
    runtime without its version) and its objects are cached there per compiler (path, args, version,
    file stamp) + flags + compile-related environment; extraction and installs are atomic;
  - `GOCPP_CXX`/`CXX` may carry arguments, split like cmd/go splits `CC` (one place, `cxxFromEnv`, for
    host and cross targets);
  - `gocpp run` forwards the program's exit status N; when the program dies by a signal, gocpp dies by
    the same signal (Linux; elsewhere 128+n), so the diff harness and shells see what they see for the
    bare binary; signals the parent ignores stay ignored (`nohup`); `gocpp check` stops the same way;
  - an internal error (C++ that doesn't compile or link, or an emitter "not supported yet") exits 2
    and keeps the work folder with the C++ and the compiler log;
  - the work folder is removed early on SIGTERM/SIGHUP while the program runs (Linux);
  - generated files get `-Wno-tautological-compare` (clang) or `-Wno-overflow` (g++); other warnings
    are shown;
  - starting a compiler or the program retries on ETXTBSY, like go run.
- 2026-10-14 (debug review 2.11): a shadowing rename (`v := v + i + 1` → `v_1`) makes gdb's `print v` show
  the OUTER variable, a wrong value under the Go name. Decision: fix it in cppgen before step 2.10's
  goldens depend on `v_1`: emit `int64_t gp_t0 = v + i + 1; int64_t v = gp_t0;` (shadowing in a nested
  block is legal C++; only reading the name in its own initializer is the problem) and keep the rename
  only where C++ forbids redeclaring in the same scope. Tradeoff to document: on the declaration line
  itself, gdb shows the not-yet-set inner v. Owner: a small job after 2.5 (or inside 2.10).
- 2026-10-14 (debug review 2.11): README §2.8 says `objdump -l`; the check uses Go's debug/dwarf (no
  external tool). Line-table check on every phase-gated run test: Phase 03 (needs `gocpp build` + run).
- 2026-10-14: Job 2.11 (the debug test, `tests/debug/` + `tests/debug.go`) committed, clean after 2 review
  rounds: gdb batch sessions (breakpoints by .gp line, `next` in .gp order, locals by Go names, `bt`)
  and a DWARF line-table check (Go's debug/dwarf) on both compilers, ~1 s. `make ci` sets
  `GOCPP_REQUIRE_GDB=1` (a missing gdb or a ptrace refusal fails instead of skipping). The driver
  now passes `-gno-column-info` (the DWARF columns were C++ columns).
- 2026-10-14 (manager, from the 2.5 review): the evaluation-order pass keeps gc's order between calls and
  everything else (also panics), the one-byte fmt copies, `&&`/`||` as units and `len`, but not the
  order between two panicking operations with no call between them: that is divergence D22 in
  docs/spec/language.md (only which panic message appears can differ).
- 2026-10-14: Job 2.5 (the evaluation-order pass, `internal/cppgen/order.go`) committed, clean after 2
  review rounds. It copies gc's walk/order.go: calls are units hoisted into `gp_` temporaries in Go's
  order when a neighbour could see the difference; also units: `len(x)` when x could change, `&&`/`||`
  with effects, fmt arguments of one-byte types that gc copies (gc's OCONVIFACE/addrTemp rule; Phase 03
  extends it to every interface conversion); panicking operations keep their order against calls but
  not against each other (divergence D22). Every call must pass the pass (`fg.ordered`), or it is an
  internal error. Random differential test kept in the repo, opt-in: `GOCPP_RANDOM_TESTS=1` (with
  `_N`, `_SEED`, `_DIR`; extra C++ flags through `GOCPP_TEST_CXXFLAGS`); ~8,000 programs matched Go on
  both compilers incl. -O2 + ASan/UBSan. This is the random test step 2.10 asked to keep. Also fixed on
  the way: untyped shift counts are uint like gc; the emitter accepts exactly the Printf directives the
  gate allows (`%z` prints `%!z(int=5)`). Known: fmtshim names int64/uint64/uintptr `int`/`uint`
  (Phase 03 type tags).
- 2026-10-14: Job 2.10a (the Phase 02 run and diff tests) committed, clean after 3 review rounds: 30 new
  tests/run and 38 new tests/diff programs (34 run and 41 diff tests at Phase 02 in all); with job 2.5 in,
  every one passes on g++ and clang++, also with `-asan`. `GOCPP_RUN_VS_GO=1` checks every tests/run
  expectation against real Go (34/34). Left for 2.10b: the 3 golden tests (after the shadowed-name fix).
  For Phase 03: a `g(x, f())` diff test where f changes x (needs package variables or pointers).
