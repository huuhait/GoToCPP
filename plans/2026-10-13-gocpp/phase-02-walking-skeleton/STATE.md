# Phase 02: Walking Skeleton · STATE

**Status:** `[~]` in progress (step 2.7, the runtime, started early: it needs nothing from Phase 01)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [ ] 2.1 Emitter skeleton (writer, `#line` with absolute paths repeated where needed, prologue, flat namespace, prototypes, main wrapper)
- [ ] 2.2 Names
  - [ ] C++ keywords, reserved names, user names starting with `gp_`
  - [ ] the runtime's macro names (`EOF`, `errno`, `assert`, `stdin`, …)
  - [ ] `gp_` temporaries
  - [ ] shadowing renames (`v := v`, `x := x + 1`)
- [ ] 2.3 Expressions
  - [ ] constants via `go/constant` (all basic types, big values)
  - [ ] string literal escaping (octal, never `\x`)
  - [ ] operators (incl. `&^`, unary `^`) + **small-integer cast rule** (int8/int16/uint8/uint16)
  - [ ] `gp::div/rem/shl/shr/f2i` (return the Go type; `MIN/-1`, `MIN%-1`, per-type float→int)
  - [ ] conversions between basic types
  - [ ] calls
- [ ] 2.4 Statements
  - [ ] `var`, `:=`, `=`, parallel assignment, op-assign, `++/--`
  - [ ] `if` with init
  - [ ] `for` (all forms; hidden counter for `range` over int; body block before labels)
  - [ ] `switch` via case index + native switch (`fallthrough`)
  - [ ] labeled `break`/`continue`, `goto` (temporaries in blocks)
  - [ ] `return` (single and multiple)
- [ ] 2.5 Minimal evaluation-order pass (incl. loop-condition / `else if` / `case` shapes)
- [ ] 2.6 Functions: params, multiple results (`std::tuple`), prototypes
- [~] 2.7 Runtime v0 (`gp::string` minimal, helpers, panic as exception, print/println, unbuffered-order stdout, fmtshim, run_main, OS layer `runtime/src/os/` + no-POSIX-outside check) + C++ unit tests
- [ ] 2.8 Driver: `build`, `run`, `emit`, `-x`, `-work`, `-asan`, compiler detection, embedded runtime
- [ ] 2.9 Internal-error path for C++ compile failures
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
