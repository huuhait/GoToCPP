# Phase 03: Core Language · STATE

**Status:** `[~]` in progress (started 2026-10-14 after M1)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike (2026-10-13): `shared_ptr` aliasing for interior pointers, non-owning global pointers, `make_shared<T[]>` element pointers, designated init, defaulted `==`; compiled and run on GCC 15 + Clang 21
- [x] 3.1 Refactor into `lower` → `cppir` → printer (Phase 02 tests stay green)
- [~] 3.2 Runtime ownership core (`gp::ptr`, `gp::new_`, aliasing helpers, nil checks, static owners for globals, `weak` intrinsic, no plain-named members on runtime types)
- [ ] 3.3 Boxing analysis + owner table
  - [ ] `&x`, `&x.f`, `&x.arr[i]`, pointer-method calls (incl. on fields), `arr[:]`
  - [ ] captured + changed (incl. field/element writes, `++`, op-assign)
  - [ ] parameters, receivers, named results
  - [ ] `&p.f`, `&s[i]`, `&arr[i]`, `&global`
- [~] 3.4 Strings (runtime, literals, slicing, concat, compare, UTF-8 range, conversions)
- [ ] 3.5 Slices (make, append + growth formula, sub-slice, copy, nil, clear, slice→array; `cap >= len` only)
- [ ] 3.6 Arrays (`gp::array`, checked index, `==`) + range rules (hidden counters, copy on `range arr`)
- [ ] 3.7 Maps
  - [ ] get / set / ref / lookup / erase / len / clear
  - [ ] nil-map panics
  - [ ] snapshot range
  - [ ] hashing (strings, arrays, structs, floats, pointers)
- [ ] 3.8 Structs
  - [ ] layout, zero values, `==`, hash, `gp_fields`
  - [ ] keyed literals in source order
  - [ ] anonymous structs (shared canonical header)
  - [ ] local types hoisted to namespace scope
  - [ ] dependency ordering + recursive types
- [ ] 3.9 Named non-struct types (`gp::newtype`) + implicit conversions where types differ + `switch` on underlying value
- [ ] 3.10 Methods (value receivers copy, held reference for non-local receivers, pointer receivers, method values/expressions)
- [ ] 3.11 Func values (by-value params), variadics, closures
  - [ ] capture analysis (by value vs boxed)
  - [ ] Go 1.22 per-iteration loop variables
- [ ] 3.12 Evaluation-order lowering for every construct (+ `&&`/`||`, loop/else-if/case shapes, two-phase assignment)
- [ ] 3.13 `defer` with the single-exit shape (panics run defers, panicking defers, named results)
- [ ] 3.14 Runtime checks + Go panic messages + exit code 2 + `backtrace()` trace + `-nochecks` (keeps Go's defined results)
- [ ] 3.15 Remaining builtins + `fmt` shim for composite values (slices, maps sorted, structs, `&{…}`, `String()`)
- [ ] 3.16 Package-level vars (`InitOrder`) + multiple `init()`
- [ ] 3.17 `gocppSizes` + `static_assert` sync
- [ ] 3.18 Tests: ≥ 150 run, ≥ 60 differential, sanitizer CI job

## Exit criteria

- [ ] `structs`, `wordfreq-core`, `closures` pass on GCC and Clang
- [ ] ≥ 150 run + ≥ 60 differential tests green on both compilers (incl. the audit cases)
- [ ] ASan + UBSan clean; LSan clean except marked cycle tests
- [ ] Recursive data structure test (tree, list, graph with `weak.Pointer`) passes

## Decisions / notes

- 2026-10-13: v1 uses `std::shared_ptr` for every heap block so interior pointers can share
  ownership through the aliasing constructor. An intrusive refcount is a Phase 15 option.
- 2026-10-13 (audit): value receivers and parameters are real copies (a `const&` rule was unsafe:
  wrong values under aliasing and an ASan use-after-free); `defer` uses the single-exit shape from
  this phase on; boxing counts field/element writes; `range` over arrays copies; local types are
  hoisted; `weak` is an intrinsic; implicit named-type conversions are inserted; runtime types have
  no plain-named members; `cap()` only promises `cap >= len`; the fmt shim prints composites.
- 2026-10-13 (from Phase 01 job 1.10a): `types.CheckExpr` (used for C++ template arguments) can't take
  gocpp's Sizes, so when step 3.17 changes the real sizes (string 32 B, pointers 16 B), re-evaluate
  `unsafe.Sizeof/Alignof/Offsetof` inside opaque template arguments or gate them (a test pins it).
- 2026-10-13 (emitter job 2.1): step 3.16 is partly done early: package-level variables of basic types
  (constant ones `constinit`, the others in `gp_var_init` in `InitOrder`) and `init` functions
  (`gp_init_1…n`, `run_main` gets `&gp_init_all`). 3.16 extends it to all types and lifts the gate.
  Also for this phase: a user type named `std` or `gp` would capture `std::`/`gp::` in generated code:
  reserve both names or write `::std::`/`::gp::` (from the 2.1 review).
- 2026-10-13 (manager, from the 2.1/2.3 reviews): Go's `int`/`int64` are both `int64_t` and
  `uint`/`uint64`/`uintptr` are `uint64_t`, so fmtshim names the type wrongly in error forms
  (`%!s(int=5)` for an `int64`) and `%T` would too. Decision: this phase (with `%T`/`%v` work) passes a
  type tag from the emitter to the formatting code for the types C++ can't tell apart (a small
  wrapper at the call site, only for `int64`, `uint64`, `uintptr`); not a divergence.
- 2026-10-13 (statements review 2.4): `x op= y` is emitted `x = x op y`, which evaluates the target twice;
  `a[f()] += 1` and `*p() op= y` need it once (C++ compound assignment, or `auto& r = …;` with the
  small-integer cast). `pure()`/`isLocal()` in internal/cppgen assume no call can change a local:
  closures and pointers break that. Go 1.22 gives each iteration of a 3-clause loop its own variable,
  which matters once closures exist.
- 2026-10-14 (manager, 3.2 review): freeing a long chain of `gp::ptr` recursed and crashed at 100,000
  list nodes (-O0); `~ptr()` gets a deferred-release trampoline (job 3.2), reusable by slices and maps
  of pointers (3.5, 3.7). The nil-dereference signal line is per goroutine (thread_local), printed once
  after the panic chain; a finished recover (Phase 05) clears it. For 3.14 / tests/diff: Go prints the
  field offset in `addr=` for accesses to a field at a small nonzero offset (< 4096); gocpp prints
  0x0, and the pc is masked. For 3.17: `weak.Pointer` is 24 B in gocpp (gocppSizes must special-case
  it; Go's declaration would give 16). Phase 15 notes: one shared count for all pointers to globals
  (per-package owners), and large values held alive (memory, not the object) by weak pointers.
- 2026-10-14 (3.1 review): the cppir printer prints a Lambda on one line (an expression is a string);
  step 3.11 (closures) or 3.13 (defer) must lay out non-inline lambda bodies over several lines (the
  header ends the line, the body is indented with a `#line` per .gp statement, `}` plus the rest of the
  statement at Body.End), so closure bodies stay steppable in gdb.
- 2026-10-14 (3.2 round 2): slices and maps (3.5, 3.7) must release their owners through
  `gp::detail::release` (the long-chain trampoline); recover (Phase 05) must call
  `gp::detail::clear_signal()`. The depth limit (1000) needs ~0.5–1 MB of stack: Phase 08's thread
  stacks must allow that or lower the limit.
- 2026-10-14 (3.2 round 2): every runtime type that holds an owner (pointers, slices, maps, closure cells,
  func values, interfaces) releases it through `gp::detail::release`; its depth limit is 128 (about
  50–100 KB of stack). The owner check (a non-nil pointer must have an owner) is on unless the
  `-nochecks` build defines `GOCPP_NOCHECKS` (3.14). For whoever adds `runtime.AddCleanup` /
  `SetFinalizer`: `release` is `noexcept` and runs destructors deep in a chain, so Go cleanups must be
  queued and run outside it.
- 2026-10-14: Job 3.1 (the emitter as internal/lower → internal/cppir → printer; cppgen is a thin wrapper)
  committed, clean after 2 review rounds. Output byte-identical to before over ~2,500 outputs (all .gp
  files, examples and 1,000+ random programs, with and without OutputPath). The IR has no raw-C++
  node; assignment targets are IR expressions; Lambda has real captures and specifiers. Known limits
  in the cppir doc: lambdas print on one line (3.11/3.13), Struct/Func names are plain text, no
  range-for or structured bindings.
