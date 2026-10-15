# Phase 04: C/C++ Interop (opaque) · STATE

**Status:** `[~]` in progress
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-15

## Steps

- [x] 4.1 Compiler probes: macro list (`-dM -E`) + kind probe (namespace/type/template/value, grown from Phase 01's namespace probe), cached
- [~] 4.2 C type translator (builtins, `struct_` tags, pointers, templates, constant args, dotted/member types, gocpp types inside)
- [~] 4.3 Name translation (header packages → `::name` / unqualified macros, namespaces + renames, header-and-namespace names, `::` vs `.`, macro protection with gocpp headers first)
- [ ] 4.4 Opaque value codegen (`auto`, `gp::deref`, calls, operators, constructors, `nil` → `nullptr`, untyped constants as C literals)
- [ ] 4.5 Template-vs-index rule (kind probe) + parenthesis escape
- [ ] 4.6 Composite literals of C/C++ types (positional, keyed via assignments, init lists)
- [ ] 4.7 `gp::range` (by-value temporaries, index walk with size read once) + `gp::values`
- [ ] 4.8 Opaque objects in closures/defers (boxed) + copy/move/default-construct policy + `gp::result_slot`
- [ ] 4.9 String boundary (`string_view` implicit, `std.string(s)` explicit, `gp::cstr` holder, `C.CString`, `C.GoString*`, `C.GoBytes`)
  - [ ] typed helpers: front-end rewrites `C.GoString` & co. into the hidden `gocpp/cgoh` package
- [ ] 4.10 `unsafe` + pointers (owning `unsafe.Pointer`, `SliceData`/`String`/`Slice` keep owners, `std::span`, `gp::ptr` → `T*`) + ASan test
- [ ] 4.11 Callbacks (function pointers, `std::function`, `cgo.Handle`) + panic trampolines
- [ ] 4.12 C++ exception → fatal Go-style panic; fmt shim prints C++ values (raw pointers as addresses)
- [ ] 4.13 Compile + link package `.c/.cc/.cpp/.cxx` sources
- [ ] 4.14 `extern "C"` rule (parse-as-C, `.hpp` not wrapped by default, `//gocpp:extern-c <hdr>`, `no-extern-c`, the §2.9 error cases)
- [~] 4.15 Directives → flags (`pkg-config`, `link`, `cflags`, `ldflags`) + the flag allow-list (port Go's security.go tables, `GOCPP_*_ALLOW`); tests use a test-only library with its own `.pc` (the dev shell hides missing flags)
- [ ] 4.16 Local helper headers used as packages (replaces the preamble) + `C` helpers (`C.free`)
- [ ] 4.17 Interop error reporting (`.gp` line mapping, trimming, `-cpperrors=full`)
- [ ] 4.18 Tests
  - [ ] ≥ 40 interop run tests
  - [ ] header zoo (std + SDL2 + raylib + sqlite3 + zlib) on GCC and Clang
  - [ ] audit cases: `::` vs `.`, macro collisions, C++ object in `defer`, move-only types, `gp::range` temporary
  - [ ] import model: renamed / unnamed headers, `glm`-style names, namespace renames
  - [ ] interop error tests (right `.gp` line)
- [ ] 4.19 Docs: `docs/spec/interop.md` complete + cookbook

## Exit criteria

- [ ] `cpp-std`, `sqlite`, `raylib`/`sdl`, `mixed-sources` examples pass on both compilers
- [ ] Header zoo compiles on both compilers
- [ ] ≥ 40 interop run tests green (incl. audit cases)
- [ ] Seeded C++-side errors reported at the correct `.gp` line

## Decisions / notes

- 2026-10-15 (jobs): 4.1 (probes) and 04-rt (the runtime side of §2.12: `gp::deref`,
  `gp::range`, `gp::values`, the string, span and pointer conversions, `gp::cstr` and the cgo
  helpers, with C++ tests) run in parallel; the lowering steps 4.2–4.4 follow 4.1, and 4.7, 4.9
  and 4.10 then only wire in what 04-rt built.

- 2026-10-13 (spike probe): `std::minmax(1,2)` returns `std::pair<const int &, const int &>`,
  so multi-value results from C++ go through `gp::values` to avoid dangling references.
- 2026-10-13 (audit): added the macro + kind probes (`::` vs `.`), fully qualified C names,
  macro protection, boxing of C++ objects used by closures/defers, a parse-as-C `extern "C"` rule
  (the old `<math.h>` claim was wrong; `<vector>` is what breaks), own-TU preambles, panic
  trampolines for C callbacks, by-value `gp::range` temporaries, owning `unsafe` pointers (ported
  `strings.Builder` needs them), the `gp::cstr` holder, and explicit `std.string(s)`.
- 2026-10-13 (your decision): C/C++ names come through header packages (`raylib.X`) and
  namespaces (`std.vector`) instead of `C.X`; `C` keeps only C's built-in types and the helpers.
  The cgo-style preamble is dropped from v1 (step 4.16 now covers helper headers instead).
- 2026-10-13 (directive job 1.7): flags from directives and pkg-config are checked by an allow-list
  like Go's, before any compiler or probe runs them (`gocpp check` and the editor run probes);
  `.h++`/`.inl` are C++ headers; an explicit `extern-c` wins; `extern-c` + `no-extern-c` on one
  header is an error.
- 2026-10-13 (namespace probe job 1.10c): until this phase applies `//gocpp:pkg-config` and
  `//gocpp:cflags` (through the flag allow-list, §2.9), the probe is skipped for a package with any
  such directive (all names Unknown, no include errors), so there are no false errors outside a Nix
  shell. This phase removes that rule (`TODO(phase 04)` in internal/check/nsprobe.go) and passes the
  checked flags to the probe. A gap to close here too: flags from the environment (trusted) other
  than CPATH/CPLUS_INCLUDE_PATH don't reach the probe.
- 2026-10-15: Job 4.1 (the kind probe and the macro probe, internal/interop) committed, clean after 2 review
  rounds. `ProbeKinds` answers Namespace, Type, Template, Value, ValueTemplate (function and variable
  templates: `std.make_unique`, `std.get`, `std.in_place_index`), Macro, NotFound or Unknown, with one
  compiler run per include set + flags; `ProbeNamespaces` is the old Phase 01 file (namespace answers only),
  which check and the language server keep using, so their cost is unchanged. `ProbeMacros` (`-E -dM`) keeps
  the predefined macros (they matter for 4.3's collisions). Both caches key on the flags; package flags that
  only change how messages print are dropped, and a path whose instantiation never ends is marked NotFound
  and the probe runs again without it. `decltype(P)` replaced the plan's `&P` (works for enumerators and
  objects that can't be copied). Cost: about 0.5 s (g++) / 0.6 s (clang++) for `<vector> <algorithm>
  <chrono> <iostream> <string> <filesystem>` and 50 paths, plus about 4 ms per path that isn't found, on
  g++ only; the macro probe about 45 ms. For later steps: 4.5 treats ValueTemplate as a template too; 4.3
  calls ProbeKinds with all the dotted paths of each include set (known from load.Rewrite) once, or the build
  pays one more run per include set; Phase 06's disk cache of probe results keys on the flags. Compiler
  differences are listed in the ProbeKinds doc (non-static members, injected class names: g++ answers,
  clang++ says NotFound). Doc nits left: a variadic concept is ValueTemplate, not NotFound; Result.Kinds'
  doc of gp_ names in namespace mode.
