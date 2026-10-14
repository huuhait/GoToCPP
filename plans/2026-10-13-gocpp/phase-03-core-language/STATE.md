# Phase 03: Core Language · STATE

**Status:** `[~]` in progress (started 2026-10-14 after M1)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike (2026-10-13): `shared_ptr` aliasing for interior pointers, non-owning global pointers, `make_shared<T[]>` element pointers, designated init, defaulted `==`; compiled and run on GCC 15 + Clang 21
- [x] 3.1 Refactor into `lower` → `cppir` → printer (Phase 02 tests stay green)
- [x] 3.2 Runtime ownership core (`gp::ptr`, `gp::new_`, aliasing helpers, nil checks, static owners for globals, `weak` intrinsic, no plain-named members on runtime types)
- [~] 3.3 Boxing analysis + owner table
  - [ ] `&x`, `&x.f`, `&x.arr[i]`, pointer-method calls (incl. on fields), `arr[:]`
  - [ ] captured + changed (incl. field/element writes, `++`, op-assign)
  - [ ] parameters, receivers, named results
  - [ ] `&p.f`, `&s[i]`, `&arr[i]`, `&global`
- [x] 3.4 Strings (runtime, literals, slicing, concat, compare, UTF-8 range, conversions)
- [~] 3.5 Slices (make, append + growth formula, sub-slice, copy, nil, clear, slice→array; `cap >= len` only)
- [~] 3.6 Arrays (`gp::array`, checked index, `==`) + range rules (hidden counters, copy on `range arr`)
- [~] 3.7 Maps
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
- [~] 3.14 Runtime checks + Go panic messages + exit code 2 + `backtrace()` trace + `-nochecks` (keeps Go's defined results)
- [~] 3.15 Remaining builtins + `fmt` shim for composite values (slices, maps sorted, structs, `&{…}`, `String()`)
- [ ] 3.16 Package-level vars (`InitOrder`) + multiple `init()`
- [x] 3.17 `gocppSizes` + `static_assert` sync
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
- 2026-10-14: Job 3.2 (the runtime ownership core: runtime/include/gocpp/ptr.hpp with gp::ptr, gp::new_,
  aliasing, gp::static_ptr, gp::ptr_cast, gp::weak_ptr with gp::weak_make/gp::weak_value; Go's nil
  panic with the per-goroutine signal line; the long-chain release trampoline (depth 128); the
  complete no-plain-named-members test runtime/members_test.go) committed, clean after 3 review rounds.
  The emitter maps `weak.Make(p)` / `w.Value()` to `gp::weak_make` / `gp::weak_value`.
- 2026-10-14 (3.17 review): rules for later steps: 3.6 `gp::array<T, 0>` is an empty struct (1 byte, like
  gocpp's empty struct); 3.8 fields in source order at one access level, no `[[no_unique_address]]`,
  embedded fields as data members (never base classes: the empty-base optimization would change sizes,
  Phase 05 §2.6); 3.9 `gp::newtype` has the size of the type it wraps; 3.5a merge: TestSizesTable fails
  on purpose until the `[]int` row stops being pending and `-update-sizes` is run (same for maps in 3.7,
  arrays in 3.6). `unsafe.Sizeof` of a type with a C/C++ part by value is an error until Phase 04.
- 2026-10-14: Job 3.17 (gocppSizes from one table, generated runtime/include/gocpp/sizes.hpp and
  runtime/test/layout.cpp, staleness and table tests; unsafe.Sizeof/Alignof/Offsetof re-checked inside
  C++ template arguments; an error for a type with a C/C++ part by value until Phase 04) committed, clean
  after 2 review rounds. Empty struct and `[0]T` are 1 byte (D15). Placeholders: chan 16 (Phase 08), func
  32 (3.11), interface 32 (Phase 05). Pending rows (slice 3.5, map 3.7, array 3.6) make TestSizesTable
  fail on purpose when the type lands: clear `pending` and run `-update-sizes`.
- 2026-10-14 (3.5a review, for 3.3/3.5b): a plain assignment whose right side is reachable from its left
  side (`n = n.kids[0]`, `s[0] = s[0].kids[0]`) frees the source while it is read in the generated
  struct's operator=: the emitter needs copy-then-move or a temporary there. `s = append(s, …)` must
  move `s` (std::move) when no other argument reads it: 1e7 appends 130 ms vs 170–230 ms without (Go
  50–90 ms; the rest is eager memset/mmap: a calloc-based allocator for large trivial arrays is a Phase
  15 idea).
- 2026-10-14: Job 3.5a (the slice runtime: runtime/include/gocpp/slice.hpp, bounds.hpp, src/slice.cpp;
  Go 1.25 growth and every bounds/makeslice panic byte-identical to Go; stores that could free their own
  source keep a local copy) committed, clean after 2 review rounds. Step 3.5 stays open for 3.5b (the
  emitter: slice types, make, literals, append with std::move of the target, copy, sub-slicing, range,
  &s[i], and the []byte/[]rune conversions of 3.4). Strings (3.4) switch to bounds.hpp's
  gp::check_slice when both are in.
- 2026-10-14: Job 3.4 (strings: substrings sharing the owner with bounds.hpp's Go panics, gp::concat for
  chains of 3+, range over strings with Go's DecodeRune, string(int) as Go's intstring, gc's evaluation
  order for slice expressions) committed, clean after 2 review rounds; the []byte/[]rune conversions go
  to 3.5b. Literals keep an empty owner (no contended shared count); `gp_owner()` gives
  `static_owner()` where a pointer into the bytes is made. New dev knob GOCPP_GATE_PHASE (raises the gate
  phase only; the harness sets each test's own `// phase:`), so phase-NN tests run while phase NN is
  built: with GOCPP_TEST_PHASE=03 the only failures left are slices_append and the examples closures,
  structs, wordfreq-core (they need 3.5b, 3.8, 3.11).
- 2026-10-14 (manager, 3.6a review): large values. 3.3 boxes every local variable (arrays and structs,
  also parameters and named results) whose gocppSizes size is over 128 KiB (Go's ir.MaxStackVarSize).
  By-value arguments, results and temporaries of such types (`f(a)`, `x := *p`, the `range arr` copy,
  to_array/array_keyed results) still live on the fixed C++ stack, where Go's stack grows: 3.6b/3.8 put
  temporaries of such types on the heap when cheap, else it becomes a documented divergence (a new D
  entry) before Phase 03 ends. 3.6b notes (checked with Go): `len(f())`/`cap(f())` of a nil *[3]int are 3
  without a panic; `for i, v := range p` on a nil *[3]int panics with the nil dereference, `for i := range
  p` doesn't. constinit works for array literals of literal types only (not gp::string literals, not
  keyed literals of strings/pointers): 3.6b/3.16 choose constinit vs gp_var_init by that rule.
- 2026-10-14: Job 3.6a (the array runtime: runtime/include/gocpp/array.hpp: gp::array<T,N>, the empty
  [0]T specialization, Go's ==, checked index, literals incl. keyed, to_array/to_array_ptr (relies on
  -fno-strict-aliasing, pinned by a test), slicing a boxed array) committed, clean after 2 review rounds.
  Step 3.6 stays open for 3.6b (the emitter: array types, literals, indexing, ==, range copies, &arr[i],
  arr[:], the gp_hash hook for map keys).
- 2026-10-14 (3.7a review, for 3.7b/3.12): Go's order for `m[k] op= y` (checked with go run): y is
  evaluated before the slot is taken (`gp::ref(m,k) += y` matches); `m[k] /= 0` and `%= 0` do not insert
  k (gc rewrites to `m[k] = m[k] / y`: check the divisor before `gp::ref`), `m[k] <<= negative` does insert
  k (don't check the count first); never hold `auto& r = gp::ref(m,k)` across a call that may delete k:
  evaluate y into a temporary first. Map literals: gc stores the static entries (constant key and value)
  first, then the dynamic ones in source order (`map[float64]int{negz: 1, 0: 2}` is {-0: 1}): 3.7b
  reorders. Hash protocol for 3.8/3.9: comparable structs get `gp_hash()` via `gp::hash_fields` over the
  non-blank fields (blank fields also out of ==); `gp::newtype` hashes its value. The nil-map write panic
  is a runtime.Error (Phase 05 recover).
- 2026-10-14: Job 3.7a (the map runtime: runtime/include/gocpp/map.hpp, hash.hpp, src/map.cpp: gp::map
  with Go's nil/NaN/±0/key-replacement rules, ranges over a snapshot, the gp_hash protocol (gp::array has
  it now)) committed, clean after 2 review rounds. Step 3.7 stays open for 3.7b (the emitter: map types,
  make, literals with gc's static-first order, get/set/ref/lookup/delete, range, the op= order rules).
  For 3.6b/3.7b: nested array literals need their type written (`grid{{row{1, 2}, row{3, 4}}}`; clang's
  -Wmissing-braces).
