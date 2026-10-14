# Phase 02: Walking Skeleton · STATE

**Status:** `[~]` in progress (steps 2.1, 2.2, 2.3, 2.6 and 2.7 done)
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
- [~] 2.4 Statements
  - [ ] `var`, `:=`, `=`, parallel assignment, op-assign, `++/--`
  - [ ] `if` with init
  - [ ] `for` (all forms; hidden counter for `range` over int; body block before labels)
  - [ ] `switch` via case index + native switch (`fallthrough`)
  - [ ] labeled `break`/`continue`, `goto` (temporaries in blocks)
  - [ ] `return` (single and multiple)
- [ ] 2.5 Minimal evaluation-order pass (incl. loop-condition / `else if` / `case` shapes)
- [x] 2.6 Functions: params, multiple results (`std::tuple`), prototypes
- [x] 2.7 Runtime v0 (`gp::string` minimal, helpers, panic as exception, print/println, unbuffered-order stdout, fmtshim, run_main, OS layer `runtime/src/os/` + no-POSIX-outside check) + C++ unit tests
- [~] 2.8 Driver: `build`, `run`, `emit`, `-x`, `-work`, `-asan`, compiler detection, embedded runtime
- [~] 2.9 Internal-error path for C++ compile failures
- [ ] 2.10 Tests: ≥ 25 run tests, 3 golden, ≥ 25 differential vs Go (promotion, order, shadowing, float format, float→int, stdout/stderr order)
- [ ] 2.11 Debug test: gdb batch + line-table check

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
