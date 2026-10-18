# Phase 04: C/C++ Interop (opaque) · STATE

**Status:** `[~]` in progress (4.6, 4.7 and 4.9 merged; 4.8, 4.10, 4.11, 4.16 and the closeout remain)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-16

## Steps

- [x] 4.1 Compiler probes: macro list (`-dM -E`) + kind probe (namespace/type/template/value, grown from Phase 01's namespace probe), cached
- [x] 4.2 C type translator (builtins, `struct_` tags, pointers, templates, constant args, dotted/member types, gocpp types inside)
- [x] 4.3 Name translation (header packages → `::name` / unqualified macros, namespaces + renames, header-and-namespace names, `::` vs `.`, macro protection with gocpp headers first)
- [x] 4.4 Opaque value codegen (`auto`, `gp::deref`, calls, operators, constructors, `nil` → `nullptr`, untyped constants as C literals)
- [x] 4.5 Template-vs-index rule (kind probe) + parenthesis escape
- [x] 4.6 Composite literals of C/C++ types (positional, keyed via assignments, init lists)
- [x] 4.7 `gp::range` (by-value temporaries, index walk with size read once) + `gp::values`
- [x] 4.8 Opaque objects in closures/defers (boxed) + copy/move/default-construct policy + `gp::result_slot`
- [x] 4.9 String boundary (`string_view` implicit, `std.string(s)` explicit, `gp::cstr` holder, `C.CString`, `C.GoString*`, `C.GoBytes`)
  - [x] typed helpers: front-end rewrites `C.GoString` & co. into the hidden `gocpp/cgoh` package
- [x] 4.10 `unsafe` + pointers (owning `unsafe.Pointer`, `SliceData`/`String`/`Slice` keep owners, `std::span`, `gp::ptr` → `T*`) + ASan test
- [x] 4.11 Callbacks (function pointers, `std::function`, `cgo.Handle`) + panic trampolines
- [x] 4.12 C++ exception → fatal Go-style panic; fmt shim prints C++ values (raw pointers as addresses)
- [x] 4.13 Compile + link package `.c/.cc/.cpp/.cxx` sources
- [x] 4.14 `extern "C"` rule (parse-as-C, `.hpp` not wrapped by default, `//gocpp:extern-c <hdr>`, `no-extern-c`, the §2.9 error cases)
- [x] 4.15 Directives → flags (`pkg-config`, `link`, `cflags`, `ldflags`) + the flag allow-list (port Go's security.go tables, `GOCPP_*_ALLOW`); tests use a test-only library with its own `.pc` (the dev shell hides missing flags)
- [x] 4.16 Local helper headers used as packages (replaces the preamble) + `C` helpers (`C.free`)
- [x] 4.17 Interop error reporting (`.gp` line mapping, trimming, `-cpperrors=full`)
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
- 2026-10-15: Job 4.17 (cmd/gocpp/cpperrors.go: gcc's and clang's output parsed (include stacks, instantiation
  chains, macro expansions, colors stripped); a C++ error is the user's only on a .gp line with C/C++ code or
  through such a line's instantiation/macro/include chain, printed as `./main.gp:N: C++: <message>` + the source
  line, exit 1; everything else stays "internal error" (exit 2); at most 10 errors, one per line, "(and N more
  errors in gocpp's code: -cpperrors=full shows them)"; static_asserts of interop.hpp show their own message;
  undefined references from GNU ld/gold (lld/mold from canned output) at the .gp line with a hint; `-cpperrors=full`;
  the doubled "internal error:" prefix fixed) committed, clean after 2 review rounds. The exit criterion
  "seeded C++-side errors at the correct .gp line" holds for the 6 seeds (tests/interop/cpp_error_*.gp) on both
  compilers. Accepted limits: a gocpp bug on a line that also has C/C++ code counts as the user's; a multi-line
  C/C++ expression marks all its lines.
- 2026-10-15 (4.4 review, decisions): comparisons of C/C++ values are Go bools (like cgo: `var b bool = x == 3`,
  `return p == nil`); conditions accept class types that convert implicitly to bool (std::vector<bool>'s proxy);
  `&lib.global` is allowed (static lifetime); `&i` of a per-iteration loop variable of C/C++ type waits for 4.8;
  len/cap/append of gocpp slices with C element types lower as for any slice; `v, ok := m[k]` on a C++ map is
  a user error ("assignment mismatch", hint: find/count). Accepted differences from cgo: `n << x` (gocpp value
  shifted by a C count) is an error, and `C.int + C.uint` follows C++ (cgo rejects it).
- 2026-10-15: Job 4.14 (the extern "C" rule) committed, clean after 3 review rounds. A local `.h` is wrapped when
  it parses as C (`-std=gnu17 -x c`, the package's C flags; the C compiler, or the C++ compiler in C mode when
  there is none) and its own text has no `__cplusplus`; `//gocpp:extern-c` / `no-extern-c` override, with the
  two error cases at the directive. check decides once (Package.ExternC); every probe and the build get the
  same marked includes, so they agree. The C check is cached against its dependency list (from `-MMD`, or a
  `-E -MM -MG` pass when the parse fails; a missing header is watched where the compiler would find it), and
  in the language server it runs in the prober's background (answers provisional until it returns), so a
  keystroke never waits for a compiler. Limits (documented): a guard inside an included header isn't seen; a
  `__cplusplus` in a comment counts; a header that appears earlier on the search path isn't noticed (the same
  as ProbeKinds). Nits left: a table test for missingRx/missingPlaces/includeFolders (Windows paths), one long
  line in interop.md.
- 2026-10-15 (job 4.12, accepted): the program-ending path for C++ exceptions is a terminate handler set by
  run_main (the search for a handler fails before any frame unwinds, so the trace is the throw's); no defers
  or destructors run (D26; README §2.8 updated). The 4.2 decision changed: the runtime always flushes C's
  stdout and stderr before it writes (fflush on an empty buffer costs ~3 ns), because flushing only in
  programs with headers, or only stderr, still broke the order on a shared pipe. Printing of C++ class values
  uses std::formatter when `__cpp_lib_format` is there (through a tail include `gocpp/fmtcxx.hpp` after the
  user's headers, only in programs with headers, because <format> costs ~0.2 s per file), else operator<<.
  For Phase 06: with several translation units the formatter/<< choice could differ between files (an ODR
  risk): decide there.
- 2026-10-15 (4.12 review): 4.17's user-error rule is widened: an expression whose type has C/C++ parts (a gocpp
  struct with a C++ field passed to fmt) counts as C/C++ code too (lower.HasC), so an unprintable field is the
  user's error at the .gp line. `%T` and fmt's error forms name C types as gocpp sees them (`int32` for C.int,
  the enum's C++ name, `char*`), not cgo's `main._Ctype_int`: accepted until Phase 13 (documented in interop.md).
- 2026-10-15: Job 4.4 (steps 4.4 and 4.5) committed, clean after 3 review rounds. Members of C/C++ values go
  through gp::deref; operators keep Go's operand order (order.go names only real temporaries: calls,
  constructors, constants, operator results, and the temporary a place is in as `auto&&`, so references into
  them stay alive; places like `v[i]` pass as they are; hoisted values move with a cast, never `std::move`);
  comparisons are Go bools (check rewrites them to `gp_cgoh.Cmp`); `[...]` after a Template/ValueTemplate is
  template arguments, else an index, `(x)[i]`/`(x).m` escape; member calls and C++ callables; `&x` of C/C++
  variables and globals; move-only values (unique_ptr in containers); gocpp containers of C values (len, range,
  slicing, comma-ok) lower like any gocpp container. sqlite_memory.gp passes. Left (TODO.md): `for i, x = range
  arr` into an existing C-typed variable is an internal error from check's opaque pass (older than this job);
  `const d = x + 1` is rejected only at build (lowering), not by check/the editor; check's gate calls
  `const d = h2.counter + 1` "C macros in constant expressions" although counter is a variable; a comment that
  `*p` places don't name what they point into.
- 2026-10-15 (job 4.6, accepted): keyed C literals go through `static_assert(gp::cgo::field_ok<decltype(...)>); x.f = {v};` (README §2.5
  updated; round 2 replaced the first `gp::cgo::field(...)` form so bit-fields work); g++ builds of generated code also get `-Werror=narrowing` (clang already rejects non-constant
  narrowing in braces); map literals of C++ maps: keys that are names are fields, any other keys make pairs
  (`std.map[std.string, int]{"a": 1}` → `{{"a", 1}}`; a key held in a variable is written `{{k, 1}}`); `&T{...}`
  of a C/C++ type waits for 4.10 (write `v := lib.T{...}` and `&v`); an inner literal with its type left out
  inside a C/C++ type is a braced list that C++ interprets (also for struct fields, an interop extension);
  `std.vector[*Node]{{}}` gives a null gp::ptr (C++ decides), not `&Node{}`. For 4.9: a C.cstr holder as a
  keyed-literal value would die inside the lambda: hoist or reject it.
- 2026-10-15: Job 4.12 committed, clean after 2 review rounds: a C++ exception (or a std::bad_alloc that
  gocpp's new handler didn't see) that no C++ code catches ends the program from a terminate handler, with
  the throw's trace and exit 2, running nothing (D26); fmt and print/println print C/C++ values (C numbers as
  the same-size Go types, enums as numbers, raw pointers as addresses, C++ strings as text, classes via
  std::formatter or operator<<; a clear error at the .gp line otherwise); the runtime always flushes C's
  stdio before it writes.
- 2026-10-15 (job 4.7, accepted): range over a C/C++ container gives C/C++ values for both the index (or key)
  and the element, because opaque mode can't tell a vector from a map or know the element type: with the
  cgo-like rules, `append(sorted, n)` from `for _, n := range v` (v a `std.vector[int]`) needs `int(n)`.
  examples/cpp-std is changed to write the conversions; Phase 12 (typed interop) makes the index a Go int and
  the element its real type. A std::string range gives byte index + C char (write `range string(s)` for
  runes). One variable from a C/C++ call (`p := lib.f()`) goes through `gp::values` (one extra move, or one
  copy for reference members).
- 2026-10-15 (4.6 round 2): the probes now give g++ the build's error flags (`-Wno-error` + GeneratedFlags instead
  of `-w`, because g++'s -w also turns off -Werror=narrowing), so check and build agree on narrowing in headers;
  clang keeps -w (narrowing is an error by default).
- 2026-10-16: Resumed the interrupted work. The uncommitted 4.6, 4.7 and 4.9 changes survived and were
  moved to worktrees under `/home/huuhait/GoToCPP-worktrees/`; no incomplete step was marked done.
  Pending: 4.6 round-2 review, 4.7 round-1 review, and 4.9 round-2 fixes followed by independent review.
  Re-run the manager's checks before merging; the interrupted runs are not evidence of a clean review.
- 2026-10-16: Job 4.6 merged as 99e75ad after clean round-3 review. Positional/keyed C/C++ literals,
  map-only key:value forms, bit-field initialization, Go containers holding C values, and inferred package
  ellipsis arrays are covered. Manager reran the package-array regressions on GCC/Clang normally and with
  ASan/UBSan, then `make ci` and staticcheck on dev: all exit 0. Full Phase 04 examples still depend on
  unfinished 4.7, 4.9 and 4.10; this is not completion of Phase 04.
- 2026-10-16: Job 4.9 merged as 9695323 after clean round-4 review. Typed cgo helpers, owning C.cstr
  holders through ordered/variadic calls, C++ string conversions, public editor helper names, exact C numeric
  constants and representability checks are covered. Direct and copied holders in keyed or positional
  C/C++ literals are rejected with a lifetime diagnostic; retained CString fields have positive tests.
  Manager reran make ci, staticcheck and all 71 Phase 04 interop fixtures on GCC/Clang normally and with
  ASan/UBSan: all exit 0 (284 compiler/mode subcases). Opaque arithmetic result types remain unknown rather
  than being guessed from one operand.
- 2026-10-16: Paused as requested for migration. 4.6 and 4.9 are done; 4.7 is preserved in branch
  job/04-4.7 at b3300cf, not merged or marked done. Its eight original findings passed independent probes
  and final-source CI passed, but service errors interrupted the additional literal-zero edge check and
  final approval. See ../HANDOFF.md. No later jobs, push or transfer were started.
- 2026-10-16: Job 4.7 merged as 3a34afc after a clean round-3 review. The preserved WIP branch (job/04-4.7) came
  back from the remote after the migration, so it was rebased onto 4.9 (one conflict: a header comment) rather
  than redone. Review rounds: (1) a Clang-only regression, `x := lib.f(..., func literal capturing a Go local)`,
  because the wrapper put the call in a lambda's default template argument; fixed by passing the result type from
  outside the lambda; the literal-0 edge from the handoff was real (`for range lib.f(0)` with a `const char*`
  parameter: the argument was hoisted into an int), fixed by leaving an untyped constant zero in place
  (`nullConstant`; the cost is that a literal 0 is not retained for a function returning a reference into it);
  unused range variables over C/C++ values are now reported (`unusedRangeVars`; a write counts as a use, and a
  bare name used as a literal key counts as used: both documented false negatives); (2) round 2 found a false
  "declared and not used" for a range variable used as a bare literal key, fixed with `namedAsKey`; (3) clean.
  Manager reran make ci (802 s), staticcheck and all Phase 04 interop fixtures on GCC and Clang normally and with
  ASan/UBSan: all exit 0. Known and accepted: the wrapper of an opaque `x := lib.f()` writes the call text three
  times (about 3^d growth for closures nested d deep); `std.map[int,int]{k: 3}` with a variable key is read as a
  field designator by the emitter (pre-existing, not 4.7). Full Phase 04 examples still wait for 4.8, 4.10 and
  4.11.
- 2026-10-17: Job 4.16 merged as b72404c after a clean round-3 review. Most of it already worked after 4.9, 4.13
  and 4.14 (a local .h/.hpp with or without a .c/.cpp next to it is a package; edits are picked up on the next
  build with the object cache on), so it adds folder tests in the interop harness (an interop test can be a folder
  with main.gp run as `gocpp run .`), eight tests (helper_header, helper_pair, helper_c_pair, helper_h_cpp_pair,
  helper_h_cpp_guards, helper_h_cpp_error, helper_sources, helper_pair_error) plus tests/errors/import_c_preamble,
  the docs ("Small helpers"), and a link-error hint for the one real gap the review found: a `.h` that parses as C
  is wrapped in extern "C" for the generated code, but the package's own `.cpp` includes it unwrapped, so a
  `.h` + `.cpp` pair does not link (fix: `//gocpp:no-extern-c ./x.h` or guards in the header; the hint fires only
  when an undefined symbol is in that .cpp and the file has no extern "C"). The automatic extern "C" decision is
  unchanged; whether a `.h` with a same-stem `.cpp` should be treated as C++ automatically is a TODO question.
  `C.free(unsafe.Pointer(p))` and `(*C.char)(malloc(..))` still wait for 4.10.
- 2026-10-17: Job 4.8 merged after a clean round-5 review (five review rounds; the lifetime design changed twice).
  Decisions: (1) an opaque local used by a closure or defer is boxed (a gp::ptr built in place), NEVER captured by copy;
  (2) the plan text says `std::make_shared<T>`: boxes are `gp::ptr<T>` made with allocate_shared and a custom allocator so
  non-movable types work; (3) `result_slot` members are `gp_set`/`gp_take`/`operator=` (runtime types may only have
  gp_-prefixed members): Phase 05 and ADR-011 wrote `.set`/`.take`: Phase 05's 5.10 must use the gp_ names; (4) deferred
  member calls read the object path when the call runs (a raw pointer local reassigned before return calls on the new
  pointer: documented difference from Go); a nil gocpp pointer receiver panics at the defer statement; (5) hold rule and its
  cost are in docs/spec/interop.md; NOT tracked: an address stored by a Go function it was passed to, C++ functions that keep a
  pointer (TODO.md: interprocedural escape analysis for opaque locals); (6) every `a, b := f()` of opaque results moves the
  members out of the tuple (std::get<N>(static_cast<decltype(t)&&>(t))), `gp::values` still copies reference members first;
  (7) every gocpp struct field of a C/C++ type that is not a raw pointer or a C number carries a
  static_assert(gp::cgo::field_zero<T>) at its .gp line (cost: one assert per field in generated structs); (8) job 4.10
  (unsafe/pointers) reconciles with this: addrHeld takes the ROOT identifier of an address operand (rootIdentOf), 4.10 widens
  cAddr, `&T{}` of a C type must be built in place (not gp::new_), `cpperrors.go` userFacingHeaders gets unsafe.hpp.
- 2026-10-17: Job 4.10 (unsafe and pointers) merged after five review rounds. Decisions: (1) every gocpp-side `*C/C++ T` is
  `gp::cptr<T>` (16 bytes; unsafe.Sizeof of such a variable is 16, as for every gocpp pointer, D15); members of C structs, `**C.T`
  and C++ template arguments stay raw; (2) `&T{}` of a C type is registered by address (a sharded table, `gp::detail::box_owner`) so
  a raw pointer converts back into an owner; cost: building a 1M-node list is about 6-9x a plain gocpp list; the registry never
  shrinks (retained peak memory); (3) stores into raw members record owners (keeper / `side_store` purged when it doubles); reads
  that bind a variable use `gp::cgo::load` (about 30% slower on a hot `x := v[i]` loop over a std::vector<int> at -O0, not measured
  at -O2: TODO for Phase 15); (4) compile errors where the owner is unknown, a run-time fatal when a gocpp pointer is stored into C
  memory; (5) documented limits in interop.md (C++ containers of raw pointers, struct copies into long-lived places, closures that
  outlive a copy, growing slices of C structs, cycles, a C function that retains a pointer); (6) examples/sqlite stops at the
  callback ABI: that is 4.11; (7) Phase 07: a type parameter that is a pointer to a C type will be a cptr; Phase 06 splits units:
  the registry (inline function-local statics) must stay one instance, check shared libraries with hidden visibility; nits left:
  `box_register` OOM skews a counter; no fork() in the repo today (inherited shard locks).
- 2026-10-17: Job 4.11 (callbacks) merged after two review rounds. Decisions: (1) a func is a C function pointer only when it is a named
  function or a capture-less literal (one static trampoline each, identity-stable); everything else is a C++ callable (`fn_val`);
  (2) the trampoline is `noexcept`: a panic or C++ exception leaving a callback is FATAL (exit 2, Go-style report with the C frames,
  the callback's own defers first, the outer ones never): divergence D27 from cgo (an outer `recover` can't catch it); a later phase
  can carry the panic out and re-raise at the Go call site (only the trampoline body changes); (3) user data is plain
  `unsafe.Pointer` pass-through (borrowed; cgo.Handle of Phase 05 fits as an integer in the void*; no handle table in the runtime yet);
  (4) callback pointer params are `gp::cptr<T>` from the registry/foreign owner: ~5-8x a C comparator in a 1M qsort (atomic count per
  pointer argument): a borrowed-pointer fast path is Phase 15; (5) gaps: `pkg.F` and generic instances go through `fn_val` (callable,
  not a pointer; Phase 06/07 must extend `callbackValue` to SelectorExpr), a nil func passed for a `std::function` parameter gives a
  non-empty std::function, C-created threads have no alternate stack (bare SIGSEGV on overflow) and adopted goroutine records are never
  freed (TODO for 8.x: free them at thread exit), recover inside a callback needs 5.10 (tests then), Windows untested, method values
  are callables, not pointers. Conflict note: 5.10 (panic/recover, in review) merges with 4.11 only in docs/spec/interop.md and
  language.md; the code composes (verified in a scratch merge).
