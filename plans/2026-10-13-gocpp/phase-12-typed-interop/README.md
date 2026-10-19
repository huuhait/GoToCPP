# Phase 12: Typed C++ Interop via Clang

> Opaque mode (Phase 04) works for almost all of C/C++, but its errors come late (from the C++
> compiler), editors can't autocomplete C++ APIs, and a few things can't work at all. In this
> phase gocpp **asks clang for the real types** of exactly the C/C++ names a program uses and
> feeds them to type checking, codegen and the language server.

| | |
|---|---|
| Depends on | Phases 04 and 06 (build flags, cache), Phase 11 (the language server this phase plugs C/C++ knowledge into) |
| Unlocks | C/C++ hover, go-to-definition and completion in the editor (step 12.13), Phase 13 |
| Size | large (about 6–8 weeks), done in stages |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

We don't re-implement C++ semantics (overload resolution, templates, ADL). We **let clang
answer questions** about small generated "probe" snippets, the way cgo asks gcc about C names.
The spike on 2026-10-13 showed this is fast and practical:

| Probe | clang's answer |
|---|---|
| `decltype(std::minmax(1, 2))` | `std::pair<const int &, const int &>` |
| `decltype(std::vector<long>{}.size())` | `unsigned long` |
| `decltype(fopen("x", "r"))` | `FILE *` |
| `gp_val<BUFSIZ>`, `gp_val<EOF>` (integer macro values through a template argument) | `8192`, `-1` (the `ConstantExpr` value) |
| one probe run (3 headers) | 0.12 s, 28 KB of JSON with `-ast-dump-filter` (162 MB without it) |

**Compatibility rule:** typed mode must **never reject a program that opaque mode + the C++
compiler accepted**. It only reports errors *earlier* and *better*, and generates more precise
C++. Opaque mode stays available (`-interop=opaque`), and every expression falls back to opaque
when probing can't answer. The audit found several ways a naive probe design breaks this rule;
§2.2 and §2.3 are written to avoid them.

What typed mode fixes, including the two gaps the first spike found in `edge.gp`:
- errors before the C++ compile, with gocpp-style messages;
- autocompletion/hover for C/C++ APIs in the language server (§2.9);
- **gap E1**: C macros as compile-time constants, e.g. `var buf [cstdio.BUFSIZ]byte`;
- **gap E2**: promoted members of embedded C++ types, e.g. `struct{ std.vector[int] }` then
  `mv.push_back(1)`;
- exact `.` vs `->` (no `gp::deref`), keyed C struct literals as designated initializers in
  the right order, exact template-vs-index decisions.

## 2. How it works

### 2.1 Collect what the program uses

After the Phase 01 opaque pass, list every C-rooted expression and its *shape*: called with
these argument types, used as a type, read as a value, member-accessed, used in an operator, or
instantiated with these template arguments. **Note the real scope:** `go/types` marks every gocpp
expression that touches an opaque value as invalid (`total := cstring.strlen(s) + len(s)`), so
the interop checker (§2.4) has to type all of those expressions, not just the C/C++ calls
themselves.

### 2.2 Probe translation unit (one per package)

Same includes and flags as the real build:

```cpp
#include <raylib.h>
#include <cstdio>
#include <bit>
template <auto V> struct gp_val {};    // constants: clang evaluates the template argument

// expression probes: what type does this interop expression have?
using gp_probe_0 = decltype(InitWindow(std::declval<int64_t>(), std::declval<int64_t>(), "x"));
using gp_probe_1 = decltype(std::declval<std::vector<int64_t>&>().size());  // lvalue: a variable
// constant probes
using gp_probe_2 = gp_val<BUFSIZ>;                                          // integer macro
using gp_probe_3 = gp_val<std::bit_cast<unsigned long long>((double)(TAU))>; // float macro, exact bits
constexpr auto gp_probe_4 = APP_NAME;                                       // string macro (read from the AST)
// type probes
using gp_probe_5 = SDL_Window;
```

- **Value categories must mirror the generated code.** An argument that is a gocpp *variable*,
  field, index or dereference is an lvalue, so it is probed as `std::declval<T&>()`; only real
  temporaries use `std::declval<T>()`. With rvalues everywhere, any C++ function taking `T&`
  (`swap`, `std::getline(stream, line)`) would fail to probe and be reported as an "error" that
  the real build doesn't have (reproduced in the audit).
- **Constants:** read the `value` of the `ConstantExpr` node that clang puts on the template
  argument, not the printed type name. For a macro expression the type name shows the
  expression (`gp_val<(1 << 3)>`, `gp_val<(-1)>`, `gp_val<8192UL>`), while `value` is the result
  (`8`, `-1`, `8192`, with its C++ type). A computed float is printed with only 7 digits
  (`1.570796e+00` for `TAU / 4`), so floats go through `std::bit_cast` into an **integer**
  template argument, which gives the exact bits (`TAU` → `4618760256179416344`, exactly
  6.283185307179586) and needs no float template arguments. String macros can't be template
  arguments, so they are read from the `StringLiteral` in the AST (adjacent literals already
  joined: `"My" "App"` → `"MyApp"`). All of this was checked with clang 21 during the audit.
- Untyped constants are written as their literal, so clang does **exactly** the overload resolution
  the real build will do. We read only the result.
- Run `clang++ -std=c++20 -fsyntax-only -Xclang -ast-dump=json -Xclang -ast-dump-filter=gp_probe`
  and stream-parse the JSON (always filtered; unfiltered dumps are hundreds of MB).
- **Records/enums/functions**: a second filtered dump for the names actually used gives fields
  (name, type), methods (name, signature, `const`/`static`, overloads), bases and enumerators.
- **When is a failed probe an error?** Only when the probe mirrors the generated C++ exactly. Then
  clang's diagnostic is mapped back to the `.gp` position as an early error. In every other case
  the expression silently stays opaque, and the real compile decides.
- **Probes use clang, the build may use g++.** Headers can branch on `__clang__`, so with a g++
  build, probe errors are **warnings** and the expression falls back to opaque.

### 2.3 C++ types as gocpp "foreign" types

| C++ | gocpp view |
|---|---|
| `int`, `long`, `size_t`, `double`, `char`, `bool` | `C.int`, `C.long`, `C.size_t`, … (foreign basic types) |
| `T*` | `*pkg.T` (raw pointer, e.g. `*cstdio.FILE`) |
| `const T&` (parameters, results) | `T` (bound / copied at the boundary) |
| `T&` result (`v.front()`, `std::get<0>(t)`) | an **addressable** foreign value of type `T`: `v.front() = 5` stays legal, as in opaque mode |
| struct/class | foreign named type with fields + methods (+ promoted members when embedded) |
| enum | foreign type + constants |
| `std::vector<long>` | `std.vector[int]` (foreign named type) |
| overloaded function | an overload set, resolved per call by the call probe |
| object-like macro with a constant value | a real constant (usable in array lengths, `const` declarations) |
| function-like macro | stays opaque |

Boundary conversions and **operators** follow **C++ rules** (the compatibility rule):
`cstring.strlen(s) + len(s)`, `cstring.strlen(s) > n` and `var x int = cstring.strlen(s)` are all
accepted in opaque mode, so typed mode applies C++'s usual arithmetic conversions to mixed
operands instead of Go's
strict typing. cgo's strict "explicit conversion everywhere" would break code that opaque mode
accepted. Narrowing conversions get a `gocpp vet` warning instead.

### 2.4 Type checking: an interop checker, no `go/types` fork

1. **Interop checker.** Keep stock `go/types`. After it, a gocpp pass types the opaque
   expressions using probe results, including operators and every gocpp expression that touches
   an opaque value (§2.1). This gives early errors, exact codegen and editor information. It
   also knows which header declares each name, so a C-level name used through the wrong header
   package (`raylib.printf`) gets a warning.
2. **Gap E1 (macro constants) without a fork.** Constant macros are probed *first*. Before
   `go/types` runs, each `cstdio.BUFSIZ` that is used in a constant context is replaced in the AST with
   a typed constant of the probed value, keeping its position. Then stock `go/types` sees
   `[8192]byte` and is happy.
3. **Gap E2 (embedded C++ types) without a fork.** Run `go/types` once. For each "has no field or
   method" error on a struct that embeds a C++ type, look up the member in the probed record. If
   it exists, rewrite the selector to go through the field (`mv.vector.push_back(1)`) and
   re-check. Two passes, only for files that need it.
4. **A vendored `go/types` fork stays a last resort.** The audit showed what it would cost: a
   forked `go/types` is a *different* package, so x/tools (`gcexportdata` for the language server,
   `go/analysis` and `go/ssa` for `gocpp vet`) would no longer accept its types. Vendoring also
   needs copies of `internal/buildcfg`, `internal/godebug`, `internal/goversion` and
   `internal/types/errors`, and `go/types` is about 34k lines. If it ever becomes necessary, it
   gets its own ADR, and Phases 11 and 13 must budget forking x/tools' importer and losing or
   porting the analyzers.

### 2.5 Codegen upgrades with real types

| Opaque mode (Phase 04) | Typed mode |
|---|---|
| `gp::deref(x).m` | `x->m` or `x.m` exactly |
| keyed C literal via assignments | `SDL_Rect{.x = 0, .y = 0, .w = 640, .h = 480}` in declaration order |
| `[...]` template-or-index from the kind probe | exact |
| `C.cstr(s)` needed for `const char*` | a `string` passed to a `const char*` parameter is converted automatically, **unless** the function is known to keep the pointer (`putenv`, `sqlite3_bind_text` with `SQLITE_STATIC`, …): those stay explicit (`C.cstr`/`C.CString`), from a list `gocpp vet` also uses. A string with an embedded NUL byte would be cut short silently, so vet warns where that can happen |
| reference results always copied by `gp::values` | copied only when they would dangle |

### 2.6 Caching and speed

Probe results are cached by hash(include set, flags, compiler version, probe text **and the
contents of every header the probe included**, taken from the `-MD` depfile, as in Phase 06).
Without the header contents, editing `./engine.hpp` would leave stale answers. New references
only add new probes (built in 12.7: the cache is per reference, ADR-088). Probing runs in parallel with other build work. Targets: cold < 0.5 s per
package, warm ≈ 0.

### 2.7 `gocpp bindgen <header>`

Writes a readable gocpp **view** of a C/C++ header, as comments or a stub file, for docs and
the editor. It is similar to Zig's `translate-c` or `c-for-go` output. Struct-valued macros are
shown as `var`, because Go constants must be basic types:

```go
// Code generated by gocpp bindgen from <raylib.h>. DO NOT EDIT.
// func raylib.InitWindow(width C.int, height C.int, title *C.char)
// type raylib.Color struct { r, g, b, a C.uchar }
// var raylib.RAYWHITE = raylib.Color{245, 245, 245, 255}
```

### 2.8 Requirements and modes

- Probing needs `clang++` even when the real build uses `g++`. On Linux both use libstdc++,
  so the answers almost always match (and §2.2 handles the rest). If clang is missing, gocpp warns
  once and uses opaque mode.
- For Windows builds the probes run with the Windows target and its headers
  (`--target=x86_64-w64-windows-gnu` plus the MinGW-w64 sysroot, Phase 14), because
  `<windows.h>` and friends only make sense for that target.
- `-interop=typed|opaque`. Opaque stays the default until typed mode passes the whole Phase 04
  suite, and then the default switches to typed.

### 2.9 C/C++ in the language server (without slowing it down)

The Phase 11 server answers gocpp questions in milliseconds. C/C++ knowledge must not change that,
so it is added **only in the background**:
- **Header symbol index**, once per distinct include set, stored in the on-disk cache. A filtered
  dump can only return names you already know, and AST dumps contain no macros, so the index is
  built from `-Xclang -ast-list` (every declared name: 20,924 for `<vector>`+`<algorithm>`+`<cstdio>`
  in 0.13 s) plus `-E -dM` (every macro: 1,639 in 0.02 s), both measured during the audit. Details
  come from filtered dumps on demand. Completion after a header package (`raylib.`) or a namespace
  (`std.`) is then a plain index lookup.
- **Hover and go-to-definition** for C/C++ names use the probe results (§2.2) plus the `loc` field
  of clang's JSON (header file + line).
- **Never blocking**: if the index or a probe isn't ready, the server answers without it, and the
  C/C++ part shows up a moment later. gocpp diagnostics never wait for clang.
- Member completion on C++ values (`v.` where `v` is a `std::vector`) uses the probe type of `v`
  plus the record's member list, or clang's code completion as a fallback for hard cases
  (templates). That is a cc1 option, so it is passed as `-Xclang -code-completion-at=file:line:col`
  (without `-Xclang`, clang rejects it). Its output also lists private members, marked
  `(Inaccessible)`, which are filtered out.
- **The C/C++ row of the speed contract** (moved here from Phase 11): C/C++ hover and completion
  data arrive in the background less than 2 s after opening a file and **never** delay the other
  Phase 11 budgets. A test runs the editor benchmarks with clang made artificially slow and checks
  that no request waits for it.

## 3. Steps

1. **12.1 Reference collector** (expression shapes, incl. operators and every expression that
   touches an opaque value).
2. **12.2 Probe generator + runner + streaming JSON reader**: lvalue/rvalue arguments that mirror
   the generated code; integer, float (`bit_cast`) and string constant probes read from
   `ConstantExpr`/literals; type probes.
3. **12.3 Record/enum/function extraction** (filtered per used name).
4. **12.4 C++ → foreign type mapper** (`T&` results as addressable values, templates, overload sets).
5. **12.5 Interop checker**: early errors only for exact-mirror probes, operator typing with C++
   conversions, g++ builds turn probe errors into warnings, compatibility rule.
6. **12.6 Codegen upgrades** (§2.5, incl. the "keeps the pointer" list for automatic `const char*`).
7. **12.7 Cache + incremental probing + parallelism** (header contents in the key).
8. **12.8 Gap E1**: constant macros probed first and substituted before `go/types`.
9. **12.9 Gap E2**: second type-check pass that routes promoted C++ members through the field.
10. **12.10 `gocpp bindgen`** (struct-valued macros as `var`).
11. **12.11 Modes + clang-missing fallback**; switch the default when ready.
12. **12.12 Tests**: the whole Phase 04 suite + header zoo in **both** modes; ≥ 30 early-error tests;
    E1/E2 tests; `T&` parameters and results; mixed C/gocpp arithmetic; float and string macros;
    a header edit invalidating probes; probe-speed benchmarks; clang 17–21 JSON compatibility.
13. **12.13 Language-server integration** (§2.9): `-ast-list` + `-dM` header index, header-package
    and namespace completion, member completion on C++ values, hover/definition into headers, the
    C/C++ budget row and the slow-clang test.

## 4. Deliverables

- Probe engine, foreign type mapping, interop checker, codegen upgrades, `bindgen`
- Gap E1 and E2 solutions on stock `go/types`
- C/C++ intelligence in the language server

## 5. Exit criteria

- The Phase 04 suite and header zoo pass in typed **and** opaque mode.
- `[cstdio.BUFSIZ]byte` (E1) and embedded `std.vector[int]` promotion (E2) work in typed mode, with
  stock `go/types`.
- ≥ 30 early-error tests give clear gocpp-style messages before the C++ compile, and no program
  that compiles in opaque mode gets a typed-mode error (compatibility tests: `T&` parameters,
  assignable `T&` results, mixed arithmetic).
- Warm builds add < 50 ms; cold probing < 0.5 s per example package.
- `bindgen` output for `raylib.h`, `sqlite3.h` and `SDL.h` is reviewed.
- In the editor, `raylib.` and `std.` completion works on the interop examples, the C/C++ data arrives
  in < 2 s, no request waits on clang (slow-clang test), and the Phase 11 budgets still pass.

## 6. Risks

| Risk | Mitigation |
|---|---|
| C++ is huge (templates, ADL, SFINAE, operators) | Never model it: probe it. Per-expression fallback to opaque |
| Probes disagree with the real build (value categories, compiler differences) | Mirror the generated code exactly; treat non-mirror and g++-build probe failures as warnings/fallback |
| The interop checker's real scope (every expression touching opaque values) is larger than it looks | Planned in §2.1 and sized at 6–8 weeks; anything it can't type stays opaque |
| A `go/types` fork would cut us off from x/tools | Not planned: E1 and E2 are solved on stock `go/types`; a fork needs its own ADR |
| clang JSON format changes between versions | Tolerant reader; CI against several clang versions |
| clang work makes the editor slow | Strictly background + cached; budgets re-measured with C/C++ features on; slow-clang test |

## 7. Built so far: steps 12.1 to 12.6 (API for 12.7 and later)

The contract is [docs/spec/typed-interop.md](../../../docs/spec/typed-interop.md). In short:

- `internal/typedinterop.Analyze(ctx, pkg, fset, Options)` runs the collector, the kind probe and
  the probe for one checked package and returns a `Report{Set, Table, Includes, Kinds}`.
  Errors wrap `typedinterop.ErrNoClang` when clang++ is missing (12.11 falls back to opaque).
- **12.3 (records, enums, functions)**, done: `ctype` (the structured type and its parser),
  `Result.Written` (typedef names kept), and `probe.Runner.Decls` (records, enums, overload sets,
  by id from filtered dumps: ADR-074). `Options.Decls` in `Analyze` runs it after the probe, a few
  rounds deep. Not done: header locations, parameter names of free functions, `explicit`.
- **12.4 (type mapper)**, done: `ftype.Mapper` maps a type or a `probe.Result` to a `go/types`
  type of the foreign view (`Mapped`, `Value`), with `Opaque` and a reason as the fallback;
  `MembersOf`, `Enumerators`, `Sig`, `Needs`/`NeedsMore`, `Value.Compat`. Results can name the
  stand-ins `gp_pr::string`, `gp_pr::slice<T>`, `gp_pr::array<T, N>`, `gp_pr::map<K, V>`:
  they map to Go's `string`, `[]T`, `[N]T`, `map[K]V`.
- **12.5 (interop checker)**, done: `internal/typedinterop/icheck` (ADR-079, spec §7).
  `icheck.Check(ctx, pkg, fset, Options)` returns errors (warnings for a g++ build) at the `.gp`
  position in Go's words, only for exact-mirror probes (`Result.Silent` says what was left to the
  C++ compiler and why), plus the typed rules from the declarations (void used as a value, `Compat`
  says No, assignment to a temporary). `probe.Result.Recovered` is the error of a probe that clang
  recovered from. The hidden command `gocpp __typed-check [-e] [-v] [-build g++|clang++]` runs it;
  the mode switch is 12.11 and the default pipeline is unchanged. Not done: narrowing warnings
  (`gocpp vet`), the wrong-header warning (needs header locations, 12.13), an incomplete type used
  as a value.
- **12.6 (code generation)**, done: `lower.Interop.SetTyped(report, cross)` (ADR-087, spec §8; the hidden
  switch `GOCPP_TYPED_INTEROP=1` in the driver until 12.11 gives the mode flag) makes the lowering write
  `x.m` / `gp::nonnull(p)->m` / `gp::smartnn(sp)->m` (only where the compiler says the class lacks the member) from the C++ type, designated initializers in declaration order (Go's
  order of evaluation kept), Go strings for `const char*` and `std::string` parameters (the keeps-the-pointer list
  in `lower/keeps.go`), C++'s conversions between C++ and gocpp numbers (`gp::cgo::as<T>`), assignable and
  addressable `T&` results, and a Go string as the key of a `std::map`. Every rule falls back to the opaque
  lowering where the probe has no answer; the default output is unchanged. Not done: `T&` parameters bound to
  Go lvalues, the members of global values (`std.cout.x`: no probe of the value alone), hoisting of C strings
  out of loops, an exact template-vs-index decision beyond the kind probe's, the mode flag and the default
  switch (12.11).
- **12.7 (cache + incremental + parallel probing)**, done: ADR-088, spec §9. A probe file is a preamble and one
  *unit* per reference; results are cached per unit on disk under (key of the preamble, hash of the contents of
  its headers from `-MD`, unit text without its number), declaration parts and the refusals of dumps over the limit
  the same way (`probe.{cache,units,declcache,limiter,aux}.go`). One edited line runs the unit that changed; the
  units that are not cached go in batches (>= 400 units, <= `Jobs()` of them, `GOCPP_PROBE_JOBS`, default
  `min(8, NumCPU/2)`, one global bound for the whole process); lock files make two processes that need the same answer
  run one compiler. `GOCPP_PROBE_CACHE=off` turns it off, `gocpp __typed-probe -cache-stats | -cache-clean` show and
  empty it, `-stable` prints a report without the run's timing so that cold and warm runs compare byte for byte,
  `-stats` prints the cache counters. `probe.Runner` keeps the results in memory as before (the same codec as the disk).
  Not done: a precompiled header of the preamble (an edit still costs one run of 0.3 s of CPU for the headers), dependencies
  per reference (an edit of any header of the package loses all its results), and `gocpp build` does not call the
  typed pipeline yet (12.11).
- **12.8 (macro constants, gap E1)**, done: ADR-097, spec §10. `check.Config.Constants` (a `check.ConstFunc` that the driver
  gives under `GOCPP_TYPED_INTEROP=1`; `typedinterop.Constants` is the implementation) is asked once per package, before
  `go/types`, for the values of the C/C++ names in an array length, a const declaration, a case or a shift count
  (`Result.Value`: `big.Int`, exact float bits, string, of the references with `Ref.WantValue`, through the 12.7
  cache); `check/constsubst.go` replaces each name in the tree by an untyped constant at the same position, so
  `[cstdio.BUFSIZ]byte`, `const tau = cmath.M_PI * 2` and `case cstdio.EOF:` type-check with stock `go/types`. A name
  that is no constant is an error where Go needs one (array length, const declaration) and is left alone elsewhere;
  a case or shift (the whole `x << N`) that Go's rules reject is taken back (opaque mode stays the arbiter); unprobed names are
  unchanged; macros of `__LINE__`/`__FILE__`/`__COUNTER__`/`__DATE__` (also through other macros) are refused; the first
  translation unit `static_assert`s each substituted value against the build compiler (clang probes, g++ may build). Not done: constexpr function calls, variable-template instances,
  `constexpr const char*` variables, `C.NAME`, a cross build (the host's values).
- Developer tool: `gocpp __typed-probe [-refs] [-decls] [patterns]` prints the references and results as
  JSON.
