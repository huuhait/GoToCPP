# Phase 03: Core Language · STATE

**Status:** `[~]` in progress (started 2026-10-14 after M1)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Spike (2026-10-13): `shared_ptr` aliasing for interior pointers, non-owning global pointers, `make_shared<T[]>` element pointers, designated init, defaulted `==`; compiled and run on GCC 15 + Clang 21
- [x] 3.1 Refactor into `lower` → `cppir` → printer (Phase 02 tests stay green)
- [x] 3.2 Runtime ownership core (`gp::ptr`, `gp::new_`, aliasing helpers, nil checks, static owners for globals, `weak` intrinsic, no plain-named members on runtime types)
- [x] 3.3 Boxing analysis + owner table
  - [x] `&x`, `&x.f`, `&x.arr[i]`, pointer-method calls (incl. on fields), `arr[:]`
  - [x] captured + changed (incl. field/element writes, `++`, op-assign)
  - [x] parameters, receivers, named results
  - [x] `&p.f`, `&s[i]`, `&arr[i]`, `&global`
- [x] 3.4 Strings (runtime, literals, slicing, concat, compare, UTF-8 range, conversions)
- [x] 3.5 Slices (make, append + growth formula, sub-slice, copy, nil, clear; `cap >= len` only) (slice→array conversions moved to 3.6b; printing slices with fmt is 3.15b)
- [x] 3.6 Arrays (`gp::array`, checked index, `==`) + range rules (hidden counters, copy on `range arr`)
- [x] 3.7 Maps
  - [ ] get / set / ref / lookup / erase / len / clear
  - [ ] nil-map panics
  - [ ] snapshot range
  - [ ] hashing (strings, arrays, structs, floats, pointers)
- [~] 3.8 Structs (job 3.8)
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
- [~] 3.13 `defer` with the single-exit shape (panics run defers, panicking defers, named results)
- [~] 3.14 Runtime checks + Go panic messages + exit code 2 + `backtrace()` trace + `-nochecks` (keeps Go's defined results) (3.14a runtime + driver done; 3.14b emitter side)
- [~] 3.15 Remaining builtins + `fmt` shim for composite values (slices, maps sorted, structs, `&{…}`, `String()`) (3.15a runtime shim done; 3.15b emitter side)
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
- 2026-10-14 (3.3 review): large parameters and results are not boxed (as in Go, whose stacks grow); a
  C++ by-value `[1<<20]int` parameter overflows the stack: 3.6b/3.8 pass such values differently (by
  the caller boxing, or a reference to a temporary copy) or document it with the large-temporaries
  divergence. 3.11 must run the boxing analysis over function literals in package-level var
  initializers too (not done today).
- 2026-10-14: Job 3.3 (the boxing analysis internal/lower/box.go with the full §2.2 rule incl. large
  locals > 128 KiB and returns changing named results; pointers to basic types and strings; gc's
  evaluation order around *p and boxed variables incl. the ascompatee "save"; Go 1.22 per-iteration
  boxes) committed, clean after 2 review rounds. &x.f, &a[i], &s[i], &T{} report their steps (3.5b,
  3.6b, 3.8); the owner table is in addr.go's doc. A boxed parameter keeps its Go name (ADR-022 shape).
  Moves into boxes are written as casts (a `std::move` call leaves a <bits/move.h> row in g++ -O0's line
  table); bind's older std::move for shadowing class temporaries should switch to the same cast.
- 2026-10-14 (manager): known g++ 15 debug-info bug: at -O0, a nested-block variable returned by name
  (NRVO) gets a wrong DWARF location, so `print` shows garbage there (clang++ is right). We don't add
  -fno-elide-constructors (it changes the generated code's copies); documented in tests/debug; Phase 13's
  `gocpp debug` may add the flag for g++ debug sessions.
- 2026-10-14 (3.14a, manager): the trace uses our own ELF symtab + DWARF .debug_line reader and Itanium
  name parser (runtime/src/trace.cpp, OS parts in src/os/), not libbacktrace. Accepted divergences, to be
  documented in docs/spec/cpp-mapping.md: Windows traces have no frames until Phase 14; clang's per-file
  lambda numbers print `func?`; methods print `main.T.M`, never `main.(*T).M` (3.14b/3.10 may fix it from
  the emitter's names); keyword-escaped names keep their `_`. The stack is recorded at every panic (Phase
  05 may make it lazy for recover-heavy code). `-nochecks` removes only the bounds, conversion and nil
  checks; division and shift panics, Go's defined results, the nil-map write, `make` and `[0]T` index
  panics stay, since they are part of the operation.
- 2026-10-14 (3.15a, manager): the fmt shim's protocol for the emitter (3.8/3.9/3.10/3.15b), documented in
  runtime/include/gocpp/fmtshim.hpp: structs get `gp_type_name` and `gp_fields = gp::fields(gp::field(&T::X,
  "X", exported), …)` with every field (blank and embedded too; `gp::field<tag>` for int64/uint64/uintptr);
  named types give `gp_value()` (+ optional `gp_value_tag`); the emitter generates member hooks
  `gp_String()`/`gp_Error()` for value method sets and statics `gp_String_ptr`/`gp_Error_ptr` for
  pointer-only ones, promoted methods included; int64/uint64/uintptr arguments are passed through
  `gp::fmtshim::typed<tag>` so `%!s(int64=5)` names the Go type. Each `gp::field` also says whether the
  field is embedded (3.15a review): Go still calls String()/Error() on the exported fields of an embedded
  unexported struct (reflect's flagEmbedRO), so 3.8 must pass that flag. Until Phase 05: a non-string panic
  value inside String() prints in the builtin print format; verbs past Phase 03 stay gated.
- 2026-10-14 (3.5b, manager): every slice element read follows gc's order (read after the statement's
  calls, like `*p`), even for locals: more temporaries, but Go's results. Moves into package or boxed slices
  on `s = append(s, …)` are kept: only a growslice overflow could expose the nil to a Phase 05 recover
  (Phase 05: re-check). Self-reachable assignments (`s[0] = s[0][1:]`) are safe for slices because
  gp::slice's operator= copies first; 3.8 must give struct operator= the same rule. tests/run/slices_append
  waits for 3.15b, which must also remove the emitter's "printing a slice/pointer" errors in expr.go call().
  `[N]T(s)` and `&arr[i]` are 3.6b; `f(s...)` for user variadics is 3.11.
- 2026-10-14: Job 3.14a (Go-style panic traces: the stack recorded in panic_error, our own ELF/DWARF line
  reader and Itanium name parser, Go names and .gp file:line, Go 1.25's 50 / `...N frames elided...` / 50
  form for deep stacks (a second full walk only past 100 frames), the `?()`/pc= fallback for stripped
  programs; `-nochecks` for build/run with its own runtime cache entry) committed, clean after 2 review
  rounds. The accepted differences are listed in docs/spec/cpp-mapping.md "Where the trace differs from
  Go's". For Phase 05 (recover): every panic past 100 frames pays the full walk, recovered or not (about
  0.4 ms at depth 1,000, 2.7 ms at 10k, 190 ms at 1M; ~64 MB held near the 4M-frame cap); make it lazy if
  recover-heavy code needs it. For Phase 14: Windows `os::stack` should be one RtlCaptureStackBackTrace call
  (Vista+ has no 62-frame limit; the chunks re-walk from the top). For 3.14b: an allocation that fails
  (`make([]string, 1<<40, 1<<41)`, huge append growth) ends with an uncaught std::bad_alloc ("terminate
  called…", exit 134); Go prints "fatal error: runtime: out of memory" and exits 2: catch it in run_main.
- 2026-10-14: Job 3.15a (the fmt shim for composite values: Go 1.25's printArg/printValue/handleMethods,
  fmtsort key order, reflect's read-only flags incl. embedded fields, String/Error and their panics,
  Sprint*, %T, the strconv float and quote mirrors in src/fmt.cpp, strconv.IsPrint generated from Go 1.25)
  committed, clean after 2 review rounds. Compile cost: hello world unchanged; each new composite type
  printed adds ~25 ms (g++) / 13 ms (clang++); keep an eye on g++ in Phase 06's build-time budget.
- 2026-10-14: Job 3.5b (slices in the emitter: literals incl. keyed, make, every append form, copy, clear,
  conversions, index/slicing/len/cap, range, &s[i] and gp::elem_ptr, gc's order for element reads and
  append-in-place, the move rule for locals, package and boxed slices, moved order temporaries) committed,
  clean after 2 review rounds. New divergence D23 (docs/spec/language.md): `print`/`println` of a slice is
  a compile error (Go prints `[len/cap]0x<address>`); internal/check/printslice.go. For Phase 07: that
  check uses Underlying(), so a type parameter whose core type is a slice (`func f[S ~[]int](s S) {
  println(s) }`) is not caught yet: use the core type. tests/run/slices_append still waits for 3.15b.
- 2026-10-14: Job 3.6b (arrays in the emitter: literals incl. keyed and [...], checked index, op=, ==, slicing
  with the right owner, &a[i], [N]T(s) and (*[N]T)(s), every range form, gc's order) committed, clean after 4
  review rounds. Range over an array is by reference only when the array is reached from a variable and
  neither the body nor a range target's operands can change or free it (calls count); else
  gp::range_array_copy. Large values (> 128 KiB): new boxed variables, `&[N]T{...}` and shadowing
  declarations are built in their box (gp::new_array, new_array_keyed, new_to_array); a by-value parameter of
  such a type is `const A&` in C++ and copied into a box at the callee's entry. D24 covers the rest (results,
  literals that don't initialize a new variable, order temporaries). print/println of arrays and structs is a
  check error (as in gc). For 3.11 (func types/values): the C++ type of `func(A) R` with a large A must be
  `gp::func<R(const A&)>`; function literals with such a parameter reuse prepare/paramBoxes (const A& + box
  at entry); 3.10's method value/expression lambdas forward such parameters by reference; 3.8 large structs
  follow the same largeVar rule. For every range iterator: the iterator must own or keep alive what it reads
  after the targets' operands run (3.7b review).
- 2026-10-14: Job 3.7b (maps in the emitter: types, nil, make (hints capped at 2^20, never panic), literals
  in gc's static-first order, get/set/ref/lookup/delete/len/clear, op= with Go's insert rules for /=, %= and
  shifts, `m[k] = append(m[k], …)` with one lookup and gc's SameSafeExpr/mapKeyTemp order (wrappedKey),
  range over a snapshot with the key copied before the targets' operands and gp::range_value after them,
  array keys and values, println of a map a check error (D23)) committed, clean after 3 review rounds.
  Accepted difference (noted in cpp-mapping.md): when a range target's call deletes the current entry or
  clears the map, Go may also blank the key it assigns ("" or 0); gocpp assigns the key as reached. For 3.8:
  struct keys of 4 or 8 bytes of plain memory go into wrappedKey; static struct literals into staticLit;
  structs need gp_hash for map keys. For 3.8/3.9: a struct or named-type value read from a map is a copy
  (like arrays), so a reference to an element of `m[k]` is never stable (stableArray/varOwner).
