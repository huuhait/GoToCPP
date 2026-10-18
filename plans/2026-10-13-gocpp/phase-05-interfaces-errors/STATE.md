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
- [x] 5.4 Conversions: concrete → interface, interface → interface
- [x] 5.5 Assertions + type switches (Go's exact panic messages, incl. nil cases)
- [x] 5.6 Interface `==`, nil rules, hashing (`map[any]V`)
- [ ] 5.7 `any`, `error`, `stdlib/errors` (`New`, `Is`, `As`, `Unwrap`, `Join`)
- [x] 5.8 Embedding
  - [x] promoted fields via selection paths
  - [x] promoted methods (aliasing pointers into embedded fields)
  - [x] fully qualified field types (`::gpkg_main::Base Base;`)
  - [x] forwarding member functions (the thunks come with job/05-5.8-iface)
  - [x] embedded pointers (nil checks); [x] embedded interfaces
- [x] 5.9 Method values / expressions on interfaces
- [x] 5.10 panic/recover
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
- 2026-10-18: Job 5.10 (panic/recover) merged after two review rounds. Decisions: (1) recover uses a thread-local TICKET (not two
  entry points: `gp::func` stays 24 bytes), set only while the defer machinery runs the calls of an unrecovered panic and only for
  calls known to be aware; named func types are aware; (2) interface methods: the `aware` flag lives in the `gp::method` descriptor
  and the itab (`method_of_aware`); OBLIGATION for 5.4/5.8/5.9: the emitter must write `gp::method_of_aware` in EVERY method table
  entry (T, *T, promoted-method forwarding thunks) whose method contains recover, else `defer i.M()` makes recover return nil; staged
  tests recover_iface_method and recover_iface_embedded with real-Go expected output are in
  /tmp/claude-1000/-home-huuhait-GoToCPP/fa18916c-20de-43da-b276-dbf712f662ca/scratchpad/job510/handoff/pending-tests/ (add them
  when 5.4 merges); (3) panics carry a gp::any; runtime errors are Go's runtime.* types; a C++ exception becomes a
  gp::CppException panic ONLY when a defer is on the stack (else Phase 04's terminate path); (4) D28: gocpp = `-gcflags=-l`
  semantics for recover through inlined callees; (5) `result_slot` members are gp_set/gp_take (ADR-011 and the README fixed);
  (6) gaps: Go-level runtime.Goexit / r.(runtime.Error) / os.Exit tests need the `runtime` and `os` packages; panics in goroutines
  are tested at C++ level only (`go` not lowered); a function with `recover()` AND `defer recover()` that is itself a deferred
  call of a panic recovers in Go but not here (TODO); fmt of a panicking String() method prints print-format floats (5.11);
  `defer f(g())` with g returning several values and f an aware func value: recover returns nil; cross-package aware funcs (Phase 06).
- 2026-10-18: Job 5.8 (embedding), NON-interface half, merged after two review rounds. The interface half (promoted methods in method
  tables/itabs, embedded interfaces; branch job/05-5.8-iface) is cherry-picked after 5.4/5.5; then 5.8 gets its [x]. Decisions:
  (1) forwarders are emitted only for structs whose method table needs them (`fwdNeed`, set by the descriptors' tables; none on
  the non-interface branch) or with `Config.AllForwarders` (tests): the iface branch's `tables()` calls `needForwarders`; its test
  must mark forwarders LATE without AllForwarders to exercise the name hiding (reviewer nit A2); (2) the forwarder is a const member
  when the method is in T's METHOD SET and a static member taking gp::ptr<T> when only *T has it (by method-set membership, not
  receiver kind: 5.4 uses `vset.Lookup`); (3) promotion from ONE embedded C/C++ type works now (Phase 12 for several); a Go member and
  a C/C++ member at the same or a shallower depth is a gate ('name the field'), also a conservative false gate when the C++ type has
  no such member (`m.ID` from Meta next to an embedded std.vector must be written `m.Meta.ID`); method EXPRESSIONS on promoted C/C++
  members are gated; (4) a store into a promoted pointer member through a gocpp pointer uses store_keep; through a value or an
  embedded pointer the error names the explicit path; (5) structs with an interface field need hashing (5.4/5.5 wrote a minimal 5.6);
  (6) the aware obligation of 5.10 applies to promoted thunks too.
- 2026-10-18: Jobs 5.4 + 5.5 (conversions, assertions, type switches AND the interface emitter that 5.1-5.3 did not include) merged
  after two review rounds. Decisions: (1) named-type descriptors are PARTIAL (kind and constant elements; slice/array/func/struct
  descriptors have null elem/key/in/fields) while unnamed composite descriptors (gp_tdc_N) are complete: 5.11/Phase 09 reflect and %v
  must not read the named ones or must complete them; (2) a minimal 5.6 exists (interface ==, hash, map[any]V, both unhashable-key
  texts): 5.6 extends it, does not duplicate it; the map-key check does not recurse into struct/array keys (TODO); (3) a C++ value
  WITH methods in an interface with methods is a compile error until Phase 12; C/C++ values go into method-less interfaces with
  foreign descriptors; Go structs with C fields can implement interfaces (a struct VALUE with C fields is not comparable through an
  interface); (4) wrapper frames are hidden from traces by `abi_tag(gp_wrap)`; (5) known differences from gc: order of conversions
  inside nested expressions (`any(g).(P).a + mod()`), devirtualized nil calls, `f := ni.M` on a nil interface (5.9); (6) promoted
  methods in a method table are a clean 'not supported yet (step 5.8)' until job/05-5.8-iface lands (cherry-pick its 3 commits next);
  its test recover_iface_embedded is parked in scratchpad/job54/pending-5.8/; (7) gaps: fmt of interface values/errors (5.7/5.11), other
  packages' interfaces (Phase 06), channels (Phase 08), interface methods with C/C++ types in the signature; thunk and wrapper
  parameters are named gp_self/gp_p/gp_a0 (user types may be named self, p, a0).
- 2026-10-18: Job 5.8 interface half merged (promoted methods in tables/itabs, embedded interfaces, anonymous structs): 5.8 is DONE.
  Decisions: (1) `tables()` calls `needForwarders`: only structs that go into a method table get forwarders (~1000 lines per struct for a
  100-method embedded type); (2) a method promoted from an embedded INTERFACE is `method_of_aware` (the dynamic method is unknown;
  the ticket flows to the inner icall which clears it unless that method is aware); (3) method-set membership decides (a type embedding
  a pointer-method-only type has no methods by value); (4) KNOWN 5.4 BUG (own ticket): an interface method named like a type it uses
  (`Item() Item`) -> g++ 'changes meaning': qualify the type in the interface struct as Fields/Params do (TODO); (5) examples/shapes
  needs 5.11 (%T).
- 2026-10-18: Jobs 5.9 and 5.6 (completion) merged after one review round. Decisions: (1) the nil/empty-map unhashable-key text is made by
  hashing the key once in a thread-local 'check' mode (`gp::check_key`, `detail::key_checking`); types that can panic carry
  `gp_hash_panics` (gp::iface, arrays of such, emitted structs with an interface in a non-blank field); (2) foreign descriptor names drop
  the pointee const (`char*`, not `char const*`) and nested pointers normalise; `&p` of a gocpp `*C.T` variable does not match
  `x.(**T)` (documented), `gp::array<const char*,N>` keeps its const, writing through `.(**Pt)` into a const object is UB (documented);
  (3) FIXED a dev miscompile: named containers over named structs in an interface (descriptor objects are now forward-declared in
  `descDecls`; 6.2 restructured descDecls: re-apply the forward-declaration block); (4) iteration order of map[any] is not Go's (tests
  must not depend on it); (5) gaps: channels in interfaces done by 8.3; `%T`/`%v` of method values and func values are 5.11.
