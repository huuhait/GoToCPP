# Phase 07: Generics · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike (2026-10-13): `requires`-based `gp_call_String` picks the right call form for value methods, pointer methods and pointers to value-method types (GCC 15 + Clang 21)
- [ ] 7.1 Generic functions → templates in `pkg.hpp`
- [ ] 7.2 Explicit type arguments: `Info.Instances` + `Info.Selections` (methods of generic types); syntax/`decltype` rule for C/C++ type arguments
- [ ] 7.3 Generic types + methods + instance descriptors (Go's exact qualified names: `main.Pair[string,[]int]`; one hidden descriptor argument per type parameter)
- [ ] 7.4 Type-parameter operations
  - [ ] arithmetic cast back to `T`; checked `/ % << >>` (ints vs floats)
  - [ ] `gp::convert`, `len`/index/range over type sets, zero values
  - [ ] tests with `int8`, `uint8`, `float64`
- [ ] 7.5 `gp_call_<Method>` helpers (pointer check first, forwarded args, promoted methods, plain names like `get`)
- [ ] 7.6 Interfaces × generics (constraint methods, `comparable`, interface type args)
- [ ] 7.7 Generic type aliases (Go 1.24)
- [ ] 7.8 Range-over-func
  - [ ] Go's state machine (READY/PANIC/DONE/EXHAUSTED/MISSING_PANIC) + 4 exact messages
  - [ ] body as a normal closure (boxed state), never `[&]`
  - [ ] `break` / `continue` / `return` / labels / `goto`
  - [ ] `defer` inside the body → outer frame
- [ ] 7.9 Optional `-gp:concepts` output
- [ ] 7.10 Port `cmp`, `slices`, `maps`, `iter` (`//go:linkname` → native functions; Go tests in Phase 09)
- [ ] 7.11 Lift the generics gate; update the feature matrix
- [ ] 7.12 Tests: ≥ 60 run + differential (misuse panics, late `yield` under ASan, small ints, `%T`); compile-time measurement

## Exit criteria

- [ ] `generics` example passes on GCC and Clang
- [ ] ≥ 60 generic tests + differential green (incl. `int8`/`uint8` arithmetic, division by zero)
- [ ] Range-over-func matches Go (jumps, `defer`, all four misuse panics); late `yield` ASan-clean
- [ ] `%T` of generic instances matches Go
- [ ] Compile-time overhead measured and noted below

## Decisions / notes

- 2026-10-13: Always pass explicit template arguments (from `go/types`), never rely on C++
  template argument deduction.
- 2026-10-13 (audit): fixed after review. The range-over-func lowering now copies Go's state
  machine and uses a safe closure; generic arithmetic follows Go (cast back to `T`, checked
  division/shifts); `gp_call` handles promoted and plain-named methods; C/C++ type arguments have
  their own rule (no `Instances` entry); `%T` names use Go's exact format; ported Go test suites
  moved to Phase 09 (they need `testing`).
