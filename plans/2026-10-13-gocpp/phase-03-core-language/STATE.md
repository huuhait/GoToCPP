# Phase 03: Core Language · STATE

**Status:** `[x]` done (started 2026-10-14 after M1, finished 2026-10-15: M2 reached)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-15

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
- [x] 3.8 Structs
  - [x] layout, zero values, `==`, hash, `gp_fields`
  - [x] keyed literals in source order
  - [x] anonymous structs (canonical names; the shared header is Phase 06)
  - [x] local types hoisted to namespace scope
  - [x] dependency ordering + recursive types
- [x] 3.9 Named non-struct types (`gp::newtype`) + implicit conversions where types differ + `switch` on underlying value
- [x] 3.10 Methods (3.10a methods, 3.10b method values and expressions) (value receivers copy, held reference for non-local receivers, pointer receivers, method values/expressions)
- [x] 3.11 Func values (by-value params), variadics, closures
  - [x] capture analysis (by value vs boxed; large captures shared)
  - [x] Go 1.22 per-iteration loop variables
- [x] 3.12 Evaluation-order lowering for every construct (+ `&&`/`||`, loop/else-if/case shapes, two-phase assignment)
- [x] 3.13 `defer` with the single-exit shape (panics run defers, panicking defers, named results)
- [x] 3.14 Runtime checks + Go panic messages + exit code 2 + `backtrace()` trace + `-nochecks` (keeps Go's defined results) (3.14a runtime + driver, 3.14b check audit + fatal errors)
- [x] 3.15 Remaining builtins + `fmt` shim for composite values (slices, maps sorted, structs, `&{…}`, `String()`) (3.15a runtime shim, 3.15b emitter side, 3.15c complex numbers)
- [x] 3.16 Package-level vars (`InitOrder`) + multiple `init()`
- [x] 3.17 `gocppSizes` + `static_assert` sync
- [x] 3.18 Tests: ≥ 150 run, ≥ 60 differential, sanitizer CI job (also the weak.Pointer intrinsic, which no earlier step lowered)

## Exit criteria

- [x] `structs`, `wordfreq-core`, `closures` pass on GCC and Clang
- [x] ≥ 150 run + ≥ 60 differential tests green on both compilers (incl. the audit cases) (157 run, 112 diff at phase 03)
- [x] ASan + UBSan clean; LSan clean except marked cycle tests (`make ci-sanitize`; `// lsan: cycle` on closures_recursive and emit/closures)
- [x] Recursive data structure test (tree, list, graph with `weak.Pointer`) passes (tests/run/recursive_structures.gp)

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
  (`gp_init_1…n`, `run_main` gets `&gp_init_all`). 3.16 extends it to all types (the gates
  featPackageVars and featInitFuncs were already Phase 3, so there was nothing to lift).
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
- 2026-10-14: Job 3.8 (structs: package, local (hoisted) and anonymous struct types, embedded fields (the
  checker keeps embedding gated to Phase 05, as planned) and blank fields, zero values, literals in source
  order, &T{}, field access with Go's nil panics, owners for &x.f, ==, gp_hash and map keys (wrappedKey,
  staticLit), the fmt protocol (structs print with fmt now), a static_assert per struct from gocppSizes,
  self-reachable assignments copied first, large structs built in their box (gp::new_init)) committed,
  clean after 3 review rounds. Also fixed dev's two-phase rule for targets inside a plain local (`w.i,
  s[w.i] = …`, `a[0], x[a[0]] = …`: target.root). Anonymous struct names are position-free (a local type is
  keyed `main.T@T_1`); Phase 06's shared header must also key unexported field names by package. Implicit
  conversions between non-identical struct types (named vs anonymous, `type B A`) report "Phase 03 step
  3.9" everywhere (assignments, args, returns, literal elements, map keys, ==, range targets, switch). For
  3.9: do those conversions. For 3.10: method names go through MethodName(structCName, …) like fields, so a
  field and a method can't collide. examples/structs needs 3.9, 3.10, 3.11 and 3.15b.
- 2026-10-14 (3.13 review, for later phases): Phase 05: a deferred call that panics at every level of a deep
  recursion costs O(depth²) time and memory (each panic records its full stack and panic_chain walks again:
  10 s / 412 MB at depth 10,000, Go 2.9 s / 8 MB); make stacks lazy there. Phase 13: gdb `step` on a return
  in a function with defers enters gp::defer_frame::gp_run; `gocpp debug` should add skip rules for it and
  std::apply/__invoke. 3.10/3.11: a deferred method call or func value must store the receiver / func value
  as an argument evaluated at the defer statement. Phase 09: os.Exit must not unwind (no defers run).
- 2026-10-14: Job 3.13 (defer: the single-exit shape with one exit per return (at the return's line, as Go's
  traces show), gp::defer_frame (gp_push/gp_run/gp_on_panic, 64-byte inline buffer), arguments evaluated at the
  defer statement in gc's order, panics that run defers and chain like Go's printpanics incl. "[recovered,
  repanicked]" (gp::panic_identity copies gc's static-data rule and its ReassignOracle constant folding;
  pinned against real Go by TestPanics rp-*), stitched traces) committed, clean after 3 review rounds. Only
  gp::panic_error is caught by the generated handler: a foreign C++ exception or a glibc forced unwind passes
  through without running defers (Phase 05 converts C++ exceptions at Go boundaries). README §2.8's names are
  now gp_on_panic/gp_run, with one exit label per return instead of a single gp_exit. Stack: a function with
  defers uses about 160–184 B per call at -O0 (deep recursion with a defer per level stops at 40–50k levels on
  8 MiB; documented in cpp-mapping "Defer"). tests/debug's bt rule allows a run of gp::/std:: frames only when
  it ends in gp::defer_frame::gp_run.
- 2026-10-14 (3.9 review, for 3.10 and later): a named type over a runtime class publicly inherits it
  (gp::newtype's class shape), so the base's injected class names (newtype, string, slice, array, map, ptr)
  enter the named type's scope: inside its body and its methods' bodies, user names that could collide are
  written qualified (gpkg_main::array). The two newtype shapes (a value holder for numbers and bools, a public
  base for runtime classes) are one CRTP template partly specialized by std::is_class_v<U>: accepted.
- 2026-10-14: Job 3.9 (named types: gp::newtype<D, U> — a value holder with every Go operator for numbers and
  bools, a public base for runtime classes — for every underlying type, `type B A` structs, local named
  types, implicit conversions everywhere Go's assignability needs one (incl. `return f()` and `g(f())` of
  tuples), gp::convert for struct conversions (pointer fields shared via ptr_cast), switch on named types,
  named map keys) committed, clean after 2 review rounds. Not yet (documented): `[]MyByte(s)`,
  `string([]R{...})` for `type R rune`, slices/maps of structs that differ only in tags. For 3.10: keep
  g.hide(baseNames) around method bodies of named types and qualify package functions named slice, array,
  ptr, map, string or newtype; methods become members of the newtype structs. For 3.11: tupleArgs indexes
  the callee's params by tuple position — map tuple elements onto a variadic parameter's element type; the
  std::apply lambda names a package function, so func values and methods need their own path; align
  `std::apply(k, …)` (unqualified today) with the qualified converting path. Phase 05: panic_identity must
  key by Go type, not basic kind, once named panic values exist.
- 2026-10-14 (job 03-order5021): the seed-5021 random-test mismatch was a gc bug, not gocpp's: gc 1.25 folds
  ((x >> c) & d) << e with a constant 64-bit count c >= 1<<63 read as negative (generic.rules), on amd64 and
  arm64. gocpp follows the spec. TestRandomOrder's huge counts now read `gz` (always 0) so gc can't fold
  them; tests/diff/shift_huge_count.gp and order_random_seed5021.gp. Draft upstream issue (not filed):
  plans/2026-10-13-gocpp/upstream/gc-shift-count-issue.md.
- 2026-10-14 (3.11 review, manager): gp::func is gocpp's own 24-byte type (a shared_ptr<void> owner plus a
  code pointer; copies share the closure, as Go does; the owner is released through detail::release), not a
  std::function wrapper (std::function copies the closure on every copy): accepted; README §2.6's sample is
  updated. gc's call order: a callee that contains a call is evaluated before the arguments; a callee without
  one (f, fs[i], m[k]) is read after the argument calls. For 3.18: tests/run/closures_recursive.gp and
  emit/closures.gp (fib through a variable) are reference cycles: mark them as LSan cycle tests (exit
  criterion: "LSan clean except marked cycle tests"). For Phase 15: calls through gp_call (a package or boxed
  func variable) cost an atomic inc/dec: 1e8 calls take 2.3 s (clang++) / 3.1 s (g++) against 0.4 s through a
  local and 0.1 s in Go; idea: skip the extra owner when nothing in the package assigns the variable outside
  its declaration.
- 2026-10-14 (3.11 round 2, for Phase 13): gdb `step` into a closure called through a func value passes 5
  runtime lines (func.hpp operator()/gp_call_obj, shared_ptr.h get) and `finish` from it needs 3 commands to
  reach the caller; `gocpp debug` should add skip rules for runtime/include/gocpp/func.hpp and
  bits/shared_ptr*.h (with the defer_frame/std::apply rules from 3.13). Closure capture rule (README §2.2/2.6):
  a captured variable is boxed when it changes after capture OR when it is larger than 128 bytes (Go's limit;
  `boxShared`).
- 2026-10-14: Job 3.10a (methods: value receivers copied at entry (`T v = *this;`, or `const T&` when
  readOnly proves the body can't change anything), pointer receivers as static members, auto-address and
  auto-deref with Go's nil panic, the caller-local copy for non-local receivers, gc's receiver order (receiver
  calls first, receiver reads after the argument calls), promoted methods, fmt hooks gp_String/gp_Error and
  the _ptr statics, deferred method calls storing the receiver, name hiding in method bodies, trace names
  main.T.M / main.(*T).M like Go) committed, clean after 2 review rounds. 3.10b (after 3.11 lands): method
  values (`f := x.M`, the receiver copied now) and expressions (`T.M`, `(*T).PM`) on gp::func, from the design
  in methods.go's methodValue note; Go names their wrappers `main.T.M-fm` in traces.
- 2026-10-14: Job 3.11 (gp::func, func values and named func types, variadics incl. methods and tuple
  spreading, closures with capture analysis (by value, boxed when changed or over 128 bytes), Go 1.22 loop
  variables, deferred closures, closures in package-var initializers, multi-line lambda printing with #line
  per statement, Go's closure names in traces (main.main.func1, main.T.M.func1) on both compilers) committed,
  clean after 4 review rounds (the last one checked the rebase onto 3.10). For 3.10b: methods.go's gates
  still say "(Phase 03 step 3.10 after 3.11)": rename to "3.10b" when building method values/expressions.
- 2026-10-14 (3.15b, manager): complex numbers (complex64/complex128, complex/real/imag) are gated to Phase 03
  but were never built: job 3.15c builds them (Go's own * and / rules for Inf/NaN, constants, ==, map keys,
  printing, sizes). %T stays gated to Phase 05 (not in §2.10's verb list), though the shim prints it.
- 2026-10-14 (3.15b review, for 3.12): two reads that can panic in one argument list stay inside the C++ call,
  whose argument order is unspecified: `fmt.Println(s[i], arr[i])` and `g(s[i], arr[i])` panic on arr first with
  g++ ("with length 2"), where Go and clang++ panic on s first. 3.12 must hoist every panicking read except the
  last into a temporary when several share an argument list. Also for 3.12: `&[]int{1}` and `&map[K]V{}` pass
  the checker but hit an internal error in internal/lower/addr.go:80 (box the composite and take its address,
  as Go does).
- 2026-10-14: Job 3.10b (method values `f := x.M` with the receiver copied when the value is made, pointer
  method values holding the owner, method expressions `T.M`/`(*T).PM`/`(*T).M` with Go's nil-wrapper panic,
  gc's binding order via collectRecv) committed, clean after 1 review round. Documented: for `(*T).M` called
  with nil, Go's trace has an extra `<autogenerated>` frame; until Phase 05 that panic's value is a string.
- 2026-10-14: Job 3.15b (fmt printing of every Phase 03 value with Go's type tags, gc's fmt argument copy
  rule (dataWordFuncName, incl. small locals that a non-escaping closure changes and parts of them, D25),
  Sprint/Sprintln/Sprintf, Printf flags/width/precision, min/max with Go's float rules, println of
  pointers) committed, clean after 4 review rounds (the last finding, parameters of inlined func literals,
  documented in D25). All three Phase 03 exit examples (structs, wordfreq-core, closures) and
  tests/run/slices_append pass. %T stays gated to Phase 05.
- 2026-10-14 (3.14b, for later phases): the out-of-memory fatal error comes from std::set_new_handler in
  run_main, so a C++ library that catches std::bad_alloc to recover can't (documented: running out of memory
  always ends the program, as in Go). The stack-overflow report is a SIGSEGV handler on a 64 KiB alternate
  stack in src/os/linux/os.cpp (main stack only): Phase 08 needs a per-thread alternate stack and stack bounds
  and one reporter when several threads fail at once; Phase 14 needs Windows' EXCEPTION_STACK_OVERFLOW. With
  -nochecks, calling a nil func value and `(*T).M` with a nil pointer are undefined behavior (nil checks).
- 2026-10-14: Job 3.15c (complex64/complex128: gocpp's own gp::complex with gc's amd64 * and / (complex128div line
  by line), constants, conversions, complex/real/imag, ==, map keys, named complex types, println and panic
  forms, fmt printing like fmtComplex with key sorting) committed, clean after 2 review rounds. gocpp gives gc's
  amd64 GOAMD64=v1/v2 results everywhere (arm64 and v3+ fuse a*b+c in gc); every real-Go build in the tests
  now runs with GOAMD64=v1. Also fixed: std::complex's symmetric hash in hash.hpp.
- 2026-10-14: Job 3.16 (package variables of every Phase 03 type in InitOrder, constinit for constant array and
  struct literals of numbers, bools and complex values (never for vars over 128 KiB, nor keyed arrays of more
  than 1<<20 elements: clang can't build them at compile time), large package vars built in place in
  gp_var_init (also inside conversions), init() traces like Go, init-cycle error tests) committed, clean after
  3 review rounds. Left in D24: a large package var set from a call result (`var g = mk()`) or `[N]T(s)`.
- 2026-10-14: Job 3.14b (an audit of every Phase 03 check vs Go (TestPanics, also as -nochecks builds), fatal
  errors like Go's: out of memory via std::set_new_handler (no defers run; the trace chains a running panic
  through gp_run's run_mark) and stack overflow via a SIGSEGV handler on an alternate stack (main stack only,
  stack top from AT_EXECFN), -nochecks completeness (func-value nil calls, (*T).M's panicwrap, array literal
  checks; TestNoChecksCalls)) committed, clean after 2 review rounds. Cost: one thread-local store/restore per
  gp_run (+1–3.5 ns per call of a function with defers; Phase 15 may look). For Phase 15: mark
  defer_frame::gp_run [[gnu::noinline]] before LTO, since run_mark uses __builtin_return_address(0).
- 2026-10-14: Job 3.18 (the weak.Pointer intrinsic with its stdlib stub, tests/run/recursive_structures.gp, run tests
  133 → 159 incl. audit_* cases, `// lsan: cycle` and `// divergence: Dn` harness directives, `make ci-sanitize`
  (the repo's sanitizer CI job; there is no CI service config), defer_repanicked at depth 200) committed, clean
  after 2 review rounds. All four exit criteria hold (checked by the reviewer twice). For Phase 07: `type WP
  weak.Pointer[T]` is gated there and needs a newtype over gp::weak_ptr.
- 2026-10-15: Job 3.12 (evaluation order for every construct: gc's walk/order.go copied for assignments
  (ascompatee, safeExpr of the targets, the late targets), map reads and keys (mapKeyTemp, gc's slow and fast
  keys, the >128-byte value rule), string compares, op= and ++, defer and go (the func value first, the
  receiver hoisted), two panics in one statement; D25 for the key copies that depend on gc's inliner) committed,
  clean after 5 review rounds. Accepted D22 corners: two panics inside one late-target operand, multi-target
  assignments and declarations with closure-changed variables, non-inlined call argument order
  (TODO.md). With it every Phase 03 step is done: version.Phase = 3, and M2 is reached.
