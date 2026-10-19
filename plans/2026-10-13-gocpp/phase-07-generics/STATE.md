# Phase 07: Generics · STATE

**Status:** `[~]` in progress (7.1, 7.2 merged 2026-10-18; 7.3 next)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-18

## Steps

- [x] Spike (2026-10-13): `requires`-based `gp_call_String` picks the right call form for value methods, pointer methods and pointers to value-method types (GCC 15 + Clang 21)
- [x] 7.1 Generic functions: one ordinary C++ function PER INSTANCE (ADR-030), not templates
- [x] 7.2 Explicit type arguments: `Info.Instances` + `Info.Selections` (methods of generic types); syntax/`decltype` rule for C/C++ type arguments
- [ ] 7.3 Generic types + methods + instance descriptors (Go's exact qualified names: `main.Pair[string,[]int]`; one hidden descriptor argument per type parameter)
- [ ] 7.4 Type-parameter operations
  - [ ] arithmetic cast back to `T`; checked `/ % << >>` (ints vs floats)
  - [ ] `gp::convert`, `len`/index/range over type sets, zero values
  - [ ] tests with `int8`, `uint8`, `float64`
- [ ] 7.5 `gp_call_<Method>` helpers (pointer check first, forwarded args, promoted methods, plain names like `get`)
- [ ] 7.6 Interfaces × generics (constraint methods, `comparable`, interface type args)
- [ ] 7.7 Generic type aliases (Go 1.24)
- [x] 7.8 Range-over-func
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
- 2026-10-14 (from 3.5b): internal/check/printslice.go (D23, `println` of a slice is an error) looks at
  Underlying(); for a type parameter it must use the core type (`S ~[]int`).
- 2026-10-18: Jobs 7.1 + 7.2 (generic functions, C/C++ type arguments) merged after two review rounds (ADR-030 SUPERSEDES ADR-017's templates for functions).
  DECISION: each generic function is lowered once per instance as an ordinary C++ function by the asking package (`gp_inst_<hash>_<name>`, in the unit
  and namespace that names it; a generic of another package is instantiated from that package's SYNTAX), by `internal/lower/{subst,generics,tinfo}.go`:
  the type parameters are replaced in every type go/types recorded for the body, selections are re-looked-up on the type argument, then the normal
  lowering runs, so wrap-around, panics, `%T` are those of concrete code. Reasons: `int`/`int64` share a C++ type (templates would need hidden
  descriptors), every lowering rule depends on the Go type, defined slice types are structs. Compile time = concrete code (200 instances: g++ 15 s vs
  15.7 s by hand). CONSEQUENCES: 7.4 (type-parameter operations) and 7.5 (`gp_call_<Method>` helpers) have NOTHING LEFT for functions (tests/run/generics_*
  cover them): for 7.3 the same layer builds one struct + method set + descriptor per instance named exactly `main.Pair[string,[]int]`. C/C++ type
  arguments (7.2): written ones are baked in; others become C++ template parameters deduced by the C++ compiler; operators on them are C++'s own; go/types
  can't check these constraints, so the C++ compiler does (user errors); a generic of ANOTHER package that uses C/C++ names can't be instantiated by an
  importer (user error, Phase 12); a func value of a deduced-type instance is a user error. Gates: rows 'generic functions' and 'generic types' (Phase 07);
  lowering reports `notYet` for generic types, local types inside generic functions, std generics (`slices.Contains is not available yet (Phase 07)`).
  Known gaps/nits (TODO): `C` type parameter rejected by 'C is reserved' (reviewer: exempting FUNC type parameters = delete `checkFields(n.Type.TypeParams)` in
  the FuncDecl case of internal/load/reserved.go + update TestCheckReservedC + add tests: do it FIRST in 7.3), an anonymous struct type mentioning a C++-deduced
  type parameter is an internal error (hoisted with unsubstituted names), an untyped Go string constant into a C++-typed parameter is a C++ error,
  trace.cpp accepts only 8-hex-digit instance names (lengthened names on a hash clash print raw), only the first cross-package C++ use is reported, a
  generic function named like a C++ keyword (`class[T any]`) shows as `main.class_[...]` in traces (pre-existing escape leak), nil-pointer value-method text
  differs from gc (`invalid memory address` vs `value method ... called using nil *Base pointer`), gc binds the pointer for `x.Value` with `T = *Counter` (gocpp
  copies, per spec), N .gp files using one instance give N copies in the binary (no COMDAT folding). `unsafe.Sizeof` of string/any/slices reports C++ sizes
  (pre-existing, also non-generic). Not built: generic types (7.3), local types in generic functions, `cmp`/`slices`/`maps`/`iter` (7.10), range-over-func
  (7.8), `-gp:concepts` (7.9). `version.Phase` is still 5: generics tests run with GOCPP_TEST_PHASE=07/08.
