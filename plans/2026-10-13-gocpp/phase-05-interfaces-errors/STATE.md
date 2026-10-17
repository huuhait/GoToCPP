# Phase 05: Interfaces, Embedding, Errors, Panic/Recover · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike (2026-10-13): single-exit shape (`try` + `goto gp_exit` + LIFO defers) works on GCC 15 + Clang 21. A defer changed a named result after `return 4` (gave 40), and `recover` turned a panic into a normal return (gave -1)
- [x] 5.1 Type descriptors chosen by the Go type
  - [x] generated descriptors for named types; runtime ones for predeclared types (`int` ≠ `int64`)
  - [x] interned composite descriptors (incl. channel directions)
  - [x] foreign descriptors for C/C++ values
  - [x] method tables with package-qualified unexported names
- [x] 5.2 Interface values (`gp::iface`, boxing rules, interface structs, `gp::icall`, wrappers defined after all types)
- [x] 5.3 itab building + caches + signature checks (`static_assert`, spelling check for opaque parts)
- [ ] 5.4 Conversions: concrete → interface, interface → interface
- [ ] 5.5 Assertions + type switches (Go's exact panic messages, incl. nil cases)
- [ ] 5.6 Interface `==`, nil rules, hashing (`map[any]V`)
- [ ] 5.7 `any`, `error`, `stdlib/errors` (`New`, `Is`, `As`, `Unwrap`, `Join`)
- [ ] 5.8 Embedding
  - [ ] promoted fields via selection paths
  - [ ] promoted methods (aliasing pointers into embedded fields)
  - [ ] fully qualified field types (`::gpkg_main::Base Base;`)
  - [ ] forwarding member functions + forwarding thunks
  - [ ] embedded pointers (nil checks) and embedded interfaces
- [ ] 5.9 Method values / expressions on interfaces
- [ ] 5.10 panic/recover
  - [ ] two entry points for functions containing `recover()` (carried by `gp::func` and thunks)
  - [ ] `gp::result_slot<R>` in the single-exit shape
  - [ ] `goexit_signal` and `abi::__forced_unwind` always re-thrown
  - [ ] nested panic chain + exact output (re-panic, `panic(nil)`, addresses masked in tests)
  - [ ] `runtime.Error` values for runtime panics
  - [ ] C++ exceptions → `gp::CppException` panics
- [ ] 5.11 `fmt` on `any` (basic verbs, `Stringer`, `error`, `%T`, `%+v`, foreign values); remove `fmtshim`
- [ ] 5.12 Lift the Phase 01 gates; update the feature matrix
- [ ] 5.13 Tests: ≥ 80 run + differential vs Go (nil gotcha, `int`/`int64`/channel directions, every `recover` route, re-panic, defer order, C++ exception recovery)

## Exit criteria

- [ ] `shapes` example passes on GCC and Clang
- [ ] ≥ 80 run tests + differential tests green
- [ ] Panic first lines (addresses masked) and exit codes match Go
- [ ] Type switches and `%T` tell `int`/`int64` and channel directions apart
- [ ] C++ exception caught by `recover()` in a test
- [ ] Sanitizers clean

## Decisions / notes

- 2026-10-13: No C++ virtual functions and no inheritance for embedding. We copy Go's runtime
  design (type descriptors + itabs) so interfaces stay implicit and embedding stays non-subtyping.
- 2026-10-13 (audit): descriptors are chosen by the Go type (C++ types are shared by several Go
  types); `recover` uses two entry points so it works through func values, interface methods,
  method values and returned closures; `Goexit` is a separate non-recoverable signal; panic
  messages corrected to Go 1.25's exact texts; interface wrapper bodies moved after all types;
  embedded field types fully qualified; forwarding members for promoted methods; itab signature
  checks for C types. Now depends on Phase 04 (C++ exceptions).
- 2026-10-17: Jobs 5.1-5.3 (the RUNTIME half) merged after two clean-ish review rounds (43+ mutants, TSan, 30 %T names
  compared with real Go). The emitter (5.4 onward) must follow three design differences from §2.2/§2.3, now in
  docs/spec/cpp-mapping.md "Interfaces" (the README sections below are updated to match): (1) method signatures are
  compared as canonical TEXT, not as descriptors: the Go-side canonicaliser (5.4) must produce the signature text AND the
  name passed to td_interface() in the same form (unalias recursively; drop parameter and result names; byte -> uint8,
  rune -> int32; any -> `interface {}`; full import paths for signatures, package NAMES in the interface type's name;
  function-local types made unique; generic type arguments canonical; C types as spelled), with a test per rule against %T
  of a real Go program (the reviewer's 30-type corpus: scratchpad review-51/p7, gots2); types.TypeString is NOT canonical;
  (2) the descriptor of *T is interned at runtime from T's second method table (`gp::td_ptr<T>(&gp_td_T)`, a function-local
  static), NOT a generated `gp_td_ptr_T` object (a second generated object would break pointer identity between `[]*T`
  and the value in an interface); (3) wrapper slots follow the interface's sorted imethods (byte-order sort key, Unicode
  exportedness as Go): the emitter numbers wrappers in that order and sorts tables as type.hpp documents. Also: the emitter
  writes `T()` for every nil of an interface type (the nullptr_t constructor is explicit); passes the interface type name
  to td_interface every time (a conflicting name is fatal); uses one C++ type per Go type (the interning key omits the C++
  type); interface hooks for ==/hash (5.6) take the stored BOX (pointer descriptors take target addresses); interfaces
  never cross a shared-library boundary (one runtime per process). Not covered by the suite: icall_keep dropping the box is
  caught only under ASan, itab-cache memory orders only under TSan (8.1 adds TSan runners).
