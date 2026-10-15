# Phase 04: C/C++ Interop (opaque) · STATE

**Status:** `[~]` in progress
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-15

## Steps

- [x] 4.1 Compiler probes: macro list (`-dM -E`) + kind probe (namespace/type/template/value, grown from Phase 01's namespace probe), cached
- [x] 4.2 C type translator (builtins, `struct_` tags, pointers, templates, constant args, dotted/member types, gocpp types inside)
- [x] 4.3 Name translation (header packages → `::name` / unqualified macros, namespaces + renames, header-and-namespace names, `::` vs `.`, macro protection with gocpp headers first)
- [~] 4.4 Opaque value codegen (`auto`, `gp::deref`, calls, operators, constructors, `nil` → `nullptr`, untyped constants as C literals)
- [~] 4.5 Template-vs-index rule (kind probe) + parenthesis escape
- [ ] 4.6 Composite literals of C/C++ types (positional, keyed via assignments, init lists)
- [ ] 4.7 `gp::range` (by-value temporaries, index walk with size read once) + `gp::values`
- [ ] 4.8 Opaque objects in closures/defers (boxed) + copy/move/default-construct policy + `gp::result_slot`
- [ ] 4.9 String boundary (`string_view` implicit, `std.string(s)` explicit, `gp::cstr` holder, `C.CString`, `C.GoString*`, `C.GoBytes`)
  - [ ] typed helpers: front-end rewrites `C.GoString` & co. into the hidden `gocpp/cgoh` package
- [ ] 4.10 `unsafe` + pointers (owning `unsafe.Pointer`, `SliceData`/`String`/`Slice` keep owners, `std::span`, `gp::ptr` → `T*`) + ASan test
- [ ] 4.11 Callbacks (function pointers, `std::function`, `cgo.Handle`) + panic trampolines
- [ ] 4.12 C++ exception → fatal Go-style panic; fmt shim prints C++ values (raw pointers as addresses)
- [x] 4.13 Compile + link package `.c/.cc/.cpp/.cxx` sources
- [~] 4.14 `extern "C"` rule (parse-as-C, `.hpp` not wrapped by default, `//gocpp:extern-c <hdr>`, `no-extern-c`, the §2.9 error cases)
- [x] 4.15 Directives → flags (`pkg-config`, `link`, `cflags`, `ldflags`) + the flag allow-list (port Go's security.go tables, `GOCPP_*_ALLOW`); tests use a test-only library with its own `.pc` (the dev shell hides missing flags)
- [ ] 4.16 Local helper headers used as packages (replaces the preamble) + `C` helpers (`C.free`)
- [~] 4.17 Interop error reporting (`.gp` line mapping, trimming, `-cpperrors=full`)
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
- 2026-10-15 (04-rt review, for 4.7, 4.9, 4.10): `gp::range` keeps only prvalues by value, so a range over a
  member of a temporary (`for _, x := range f().items`, where f returns the only `*Node`) is a
  heap-use-after-free; 4.7 hoists a range expression that is neither a variable nor a prvalue into a local
  (Go evaluates it once anyway). 4.9/4.10: variadic C arguments get `.gp_ptr()` / `.gp_get()` (README §2.6).
  4.10: a string made over C memory (`unsafe.String` from a C pointer) needs a non-null owner marker, because
  a null owner promises a readable byte at `data[len]` (gp::cstr's no-copy path reads it). `gp::ptr` converts
  implicitly to `T*` (borrowed, §2.7); its conversions to `bool` and to `const volatile T*` are deleted, so
  `if (p)`, `delete p` and `p[i]` don't compile (as in Phase 03); the cost is that an `f(bool)` + `f(void*)`
  overload pair is ambiguous for a gocpp pointer (pass `p.gp_get()`). `gp::deref` also dereferences `gp::ptr`
  (README §2.5).
- 2026-10-15 (job 4.2, for 4.12 and 4.19): `fmt` writes with write(2) and C's stdio buffers, so `printf` and
  `fmt.Println` in one program come out in the wrong order when stdout is a pipe. Go with cgo has the same
  problem, but gocpp programs mix them far more often. Decision for 4.12: the fmt shim and `print`/`println`
  call `fflush(stdout)` (resp. `stderr`) before they write when the program imports a C/C++ header (cheap
  when the buffer is empty); measure it, and add a run test that mixes both through a pipe. 4.19 documents
  it, and that C literals follow C's types (`1<<40` is a `long`, which is 32-bit on Windows: write `%lld`
  with `C.longlong(x)`).
- 2026-10-15: Job 04-rt (the runtime side of §2.12 in runtime/include/gocpp/interop.hpp and src/interop.cpp:
  `gp::deref`, `gp::range` / `gp::range_keys`, `gp::values`, `gp::cstr`, `gp::cgo::{CString, GoString,
  GoStringN, GoBytes, free}` with Go 1.25's behavior, implicit `std::string_view`, `std::span` and `T*`
  conversions) committed, clean after 2 review rounds. g++ builds of generated code use
  `-Werror=conditionally-supported`, so a class object through C's `...` is an error on both compilers;
  runtime/reject_test.go checks what must not compile. For 4.10/4.11: that flag also rejects casts between
  function and object pointers (`unsafe.Pointer(C.fn)`, dlsym results, callbacks with user data): do them
  through one runtime helper built on `std::bit_cast` with a static_assert on the sizes. For 4.13:
  GeneratedFlags must stay limited to generated files, never the user's own `.c`/`.cpp` sources. Nits left:
  reject_test.go could match `<file>:<line>:<col>: error` for the cases whose error is at their own line;
  a long line in docs/spec/cpp-mapping.md:213.
- 2026-10-15 (4.2 review): decisions: C tag names are qualified (`struct ::point`); members of value paths
  go through `gp::deref` (README §2.3); conversions of C values use one runtime helper that sends float →
  integer through `gp::f2i` (a plain static_cast of an out-of-range float is UB); until 4.8, a func literal
  that uses an opaque local is "not supported yet" (capture by copy loses changes made through C++
  references); a macro used through a header package that collides with a generated name is a user error.
  For 4.7: `p := std.minmax(3, 1)` (one variable) dangles like the two-variable form (ASan
  stack-use-after-scope): a single variable from a call that returns a pair/tuple of references also goes
  through `gp::values`. For 4.9: constants at C targets follow Go's rules (`var s *C.char = "hi"`,
  `var c C.char = 200`, `C.int(2.5)` are errors, as in cgo).
- 2026-10-15: Job 4.15 (directives → flags: `//gocpp:pkg-config` (two runs, `--cflags` and `--libs`), `link`,
  `cflags`, `ldflags`; Go 1.25's cgo allow-list ported to internal/build/security.go with Go's notice; trusted
  GOCPP_CFLAGS (C and C++), GOCPP_CXXFLAGS and GOCPP_LDFLAGS; relative paths made absolute; the probes, check
  and the language server get the checked flags (pkg-config runs in the server's background); `hasFlagDirectives`
  removed, 1.10c's gap closed) committed, clean after 3 review rounds. gocpp-specific rules: a package can't
  choose the system root, the toolchain, the linker or the ABI (`-m32`, `-stdlib=`, ABI macros such as
  `_GLIBCXX_USE_CXX11_ABI`): those are choices for the whole program, made in the GOCPP_* variables; with user
  flags, gocpp's language flags (`-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -fexceptions
  -fno-fast-math`) and `-Wno-error` come last in the compile of generated C++ and of the runtime; no safeCgoName
  check (no shell: `-DMSG=\"hi\"` works); a compile or link that fails with user flags is a user error, not
  "a bug in gocpp".
- 2026-10-15 (4.13 review): the C sources use the C compiler of the C++ compiler's toolchain or `GOCPP_CC`
  (not `CC`) at `-std=gnu17` (README §2.9 updated). For 4.17: `gocpp run main.gp` leaves the folder's C/C++
  files out (like `go run main.go`); when the link then fails, say so ("gocpp run . builds the package's C
  and C++ files") instead of "a bug in gocpp". For 4.11/4.12: C files get no `-fexceptions`; on x86-64 and
  aarch64 Linux gcc and clang emit unwind tables anyway, so a panic unwinding through a C frame works in
  practice but isn't guaranteed (and the trampolines stop it at C entries anyway); a package's `-fno-exceptions`
  reaches its own .cpp files, so a panic through such a callback calls std::terminate: decide there. For 4.16:
  an inline function in a helper header is compiled with `-fwrapv` in generated code and without it in the
  user's .cpp, and the linker keeps one copy: document it. For Phase 14: check MinGW headers under
  `__STRICT_ANSI__` with `-std=c++20`.
- 2026-10-15: Job 4.13 (a package's .c and .cc/.cpp/.cxx files: found with the .gp files' build rules (name
  suffixes, `//go:build` in the leading comments, read like go/build), assembly/Fortran/SWIG/.syso are errors;
  C with the toolchain's C compiler or GOCPP_CC at `-std=gnu17`, C++ at `-std=c++20`; `-g -O0`, the -asan
  flags, `-iquote <pkg dir>`, `-include nofortify.h`, env then package flags, never gocpp's generated-code
  flags; up to 8 compiles in parallel; warnings and errors under `# pkg` with `./` paths; link order: generated,
  user, runtime objects, then the -l flags) committed, clean after 3 review rounds. The test harness now sets
  its own XDG_CACHE_HOME, so no test writes ~/.cache/gocpp (TODO closed). The language server doesn't show
  errors of C files (its file system lists only .gp files): fine for now, Phase 11 may add it.
- 2026-10-15: Job 4.2 (steps 4.2 and 4.3 plus the first part of 4.4) committed, clean after 4 review rounds.
  The build runs ProbeKinds once per include set with every dotted path (check's probe in build/run/emit asks
  for the same paths and flags, so it is one compiler run, plus one `-E -dM`); `gocpp check` and the language
  server stay in the namespace mode. Includes: rt.hpp first, then each file's headers in source order with
  `#line`; `-iquote <pkg dir>` only with a local header. Macro protection: the package's own names that are
  header macros get a `_` suffix, other generated names that are header macros are `#undef`ed after the
  includes, and a macro used through a header package that clashes with a generated name is a user error.
  Names per README §2.3 (tags `struct ::point`, members of value paths through `gp::deref`); types per §2.4.
  Values: C calls, `x := lib.f()` (auto), constructors, untyped constants as C literals, `nil` → nullptr,
  conversions through `gp::cgo::as` (float → integer via gp::f2i), switch on scalar C values
  (`gp::cgo::case_eq`), methods with C parameters. A guard (internal/lower/cguard.go) allows C/C++ values only
  in the lowered forms and places; everything else is "not supported yet (step N)", and later jobs open it up.
  Like cgo: a C/C++ value into a gocpp-typed target, or a typed gocpp value into a C-typed target, is a user
  error ("convert it, int(x)" / "C.int(x)"); arguments of C/C++ functions keep C++'s conversion rules
  (documented; Phase 12 makes them exact). Until 4.8, a func literal that uses an opaque local or a local
  struct with C/C++ parts is "not supported yet". Left for 4.9: a typed string constant passed to a C function
  (`const S string = "x"; cstdio.puts(S)`) is a gp::string argument and doesn't compile. Left for 4.17: errors
  from the C++ compiler at the boundary (static_asserts of gp::cgo::as / case_eq) print as internal errors.
- 2026-10-15 (4.17 review): decision: a C++ compile error is the user's only when it is on a .gp line that
  contains C/C++ code (a `C.`, header-package or namespace name, the first line of the statement around it, or
  a header import line for errors inside headers), or when the innermost .gp frame of its instantiation chain
  is such a line; an error on any other .gp line is gocpp's (internal), because almost every generated
  statement is #line-mapped to its .gp line (5 emitter bugs on plain Go lines all looked like user errors with
  the first rule). gcc's "in expansion of macro" note at a .gp line is the error's place. Accepted extensions of
  README §2.11: undefined references at a .gp line (from the debug info) are user errors with a hint
  (//gocpp:link or //gocpp:pkg-config, or `gocpp run .` when named files leave the folder's C/C++ files out);
  the source line follows after a tab (like gocpp check), not 8 spaces.
- 2026-10-15 (job 4.4, decisions accepted): like cgo, a typed gocpp number, bool or string next to a C/C++
  value in an operator or `op=` is a user error ("mismatched types C.int and int: convert one of them");
  untyped constants are C literals, nil is nullptr, other gocpp values (pointers, structs) follow C++ rules, a
  shift count may be any gocpp integer. This replaces docs/spec/interop.md's old "Phase 12 compatibility" line
  that mixed arithmetic follows C++ rules in opaque mode. Places with C++ types (`v[i]`, `m[k]`, `p.m`) take
  gocpp values by C++ conversions, like C function arguments; a call is never a place (Go's rule). Conditions
  must be bools (`gp::cgo::cond`); `*p` goes through `gp::cgo::star` (Go's nil panic for raw pointers); `x++` is
  `++x`; `&x` only of a C/C++ variable (not in defer/go until 4.8; `return &local` is a user error); several
  values from one C/C++ value go through `gp::values` and `std::get` (README §2.5 updated). `==` on gocpp
  structs with C/C++ fields is Phase 12 (Go compares fields; only typed interop can write that).

