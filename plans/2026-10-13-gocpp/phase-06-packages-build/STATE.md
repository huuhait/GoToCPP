# Phase 06: Packages, Modules & Build System · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] 6.1 `gocpp.mod` (modfile lax + `replace`/gocpp lines read from the syntax tree), module root, `replace`, `vendor/`
- [x] 6.2 Per-package codegen (`pkg.hpp`, one TU per `.gp`, `pkg_init.cpp`, flat escaped namespaces, stable folders)
- [x] 6.3 Header layout (gocpp headers first, public flags, fully qualified struct field types, exported constants, shared anon-type header)
- [~] 6.4 Cross-package init: Go 1.21+ global order, `gp_init_all()`, `constinit` variables, `gp::lazy<T>`
- [x] 6.5 Action graph with content hashing + `-MD` header deps + macro lists + public flags + wrapper environment
- [x] 6.6 Cache (objects, generated files, write-if-changed, API hash = export data + `pkg.hpp`, package summaries, `clean -cache`)
- [x] 6.7 Parallel executor (`-p`), `-x`, `-work`, error aggregation
- [ ] 6.8 Runtime prebuild + precompiled header
- [ ] 6.9 Link step (ordering, dedup, system libs, LTO with matching archiver)
- [x] 6.10 Build modes
  - [ ] `c-archive` + `//export` (cgo's type rules, copies) + C header + lazy init
  - [ ] `c-shared` (hidden visibility, lazy init, trampolines)
  - [ ] `cpp-lib` (lib + headers + CMake INTERFACE flags)
- [ ] 6.11 `compile_commands.json` + `gocpp export cmake`
- [ ] 6.12 CLI: `list`, `env`, `clean`, build flags, build tags
  - [ ] trusted C++ flags from the user (not from packages): `gocpp build -cxxflags=… -ldflags=…` and `GOCPP_CXXFLAGS` / `GOCPP_LDFLAGS`, which may load LLVM plugins (`-fpass-plugin=x.so`, `-fplugin=x.so`); part of the cache key (the plugin file's hash too)
  - [ ] `gocpp build -emit-llvm` (clang only): writes each translation unit's LLVM IR (`.ll`, or `.bc` with `-emit-llvm=bc`) next to the objects, for `opt -load-pass-plugin=…` and other tools; a clear error with g++
- [ ] 6.13 Cross-compilation: windows/amd64 via MinGW-w64 (`.exe` linked with `-static`) + Wine CI job; linux/arm64 best effort
  - [ ] the Windows toolchain downloaded like Go's (§2.10): `GOCPP_TOOLCHAIN=auto|local|pinned`, pinned llvm-mingw (UCRT) with SHA-256 in the binary, `GOCPP_TOOLCHAIN_URL` mirror, cache + lock, `gocpp env`/`clean -toolchain`, never started by the LSP or `check`, NixOS handled, test server tests
- [ ] 6.14 Tests: multi-package fixtures, init order vs Go, incremental-build assertions (incl. constants), build-mode consumers (incl. two c-shared libs in one process), a tiny LLVM pass plugin loaded through `-cxxflags` (it counts functions) and `-emit-llvm` output that `opt` accepts

## Exit criteria

- [ ] `multi-pkg` builds and runs on GCC and Clang
- [ ] No-op rebuild < 200 ms; body-only change → 1 TU recompiled; changed exported constant → dependents rebuilt
- [ ] Cross-package init order matches Go 1.25 (differential test)
- [ ] c-archive used from C; cpp-lib used from CMake; two c-shared libs load and unload in one process
- [ ] clangd opens generated code cleanly (manual)
- [ ] `gocpp build -os windows` makes a working `hello.exe`; OS-independent run tests pass under Wine

## Decisions / notes

- 2026-10-13 (audit): flat namespaces (`gpkg_main`), Go 1.21+ init order computed by the driver,
  API hash includes export data (folded constants), `constinit` package variables + lazy init for
  C libraries, hidden visibility for c-shared, cgo-compatible `//export` types, include order and
  public flags against macro/ABI problems, stable `-I` folders. Now depends on Phase 05.
- 2026-10-13 (your decision, Q8): Linux and Windows are the targets (no macOS). Windows test
  builds start in this phase (MinGW-w64 cross compiler from the dev shell, tests under Wine); the
  full port is Phase 14. Size raised to 4–5 weeks.
- 2026-10-13 (dev-shell review): Wine CI rules (own prefix made up front, `wineserver -p`,
  `WINEDEBUG=-all`, `winemenubuilder.exe=d` so nothing lands in the user's home); the dev shell's
  MinGW-w64 is the msvcrt flavor (UCRT isn't cached), UCRT is covered by real Windows in Phase 14.
- 2026-10-13 (docs job 0.6b): the Wine prefix is created with no display and `winewayland.drv=d`,
  because Wine's Wayland driver otherwise reaches the real desktop through the default socket.
- 2026-10-13 (your decision): C/C++ libraries from the internet are modules, and their headers are
  imported by full path (`"github.com/ocornut/imgui/imgui.h"`, package `imgui`); `<...>` is for system
  headers only. Built by gocpp from a small `cxx` recipe in gocpp.mod, no CMake (README §2.11).
- 2026-10-15 (user): the user wants both ways of using LLVM tools: loading plugins (`.so` pass or
  front-end plugins) into the C++ build, and getting the LLVM IR (`.ll`) out. Plugins come only
  from the user's own flags or environment, never from a package's `//gocpp:cflags` (Phase 04's
  allow-list keeps refusing `-fplugin` there; `GOCPP_CFLAGS_ALLOW` stays the escape hatch).
  Both need clang; g++ gets a clear error for `-emit-llvm` and its own GCC plugins via `-cxxflags`.
- 2026-10-15 (4.15 review, for 6.12/6.14): environment flags (GOCPP_CFLAGS, GOCPP_CXXFLAGS) also reach the
  compiler probes, so a plugin in them (`-fplugin=`, `-fpass-plugin=`) would load in every probe the editor
  runs, 6.14's function-counting test plugin would count probe files too, and the probe cache keys on the
  flags but not on the plugin file's contents. 6.12 drops plugin flags from the probe flags (in
  interop.probeFlags, as the message flags are dropped) or adds the plugin file's hash to the key.
- 2026-10-15 (4.15 review, for 6.13 and later targets): package flags may not change the ABI (`-m32`,
  `-mabi=`, `-mfloat-abi=`, …): Go's good list had `-mabi=lp64d` and `-mdouble-float`, and gocpp refuses
  them. Fine for the plan's targets (linux/amd64, windows/amd64, linux/arm64); a riscv64 or loong64 target
  would allow `-mabi=` values that equal the target's default.

- 2026-10-17: Job 6.1 merged after a clean round-2 review. Decisions and open points: (1) a parser of our own instead of
  `x/mod/modfile` (README §2.1 says modfile lax): `golang.org/x/mod` is not in go.mod and x/tools v0.44 needs x/mod v0.35.0,
  which is not in the offline module cache; if the user wants the real x/mod the module graph needs a network bump
  (x/tools, x/sync); the parser agrees with Go's modfile on the shared syntax (differential on ~60k inputs) and is
  stricter in documented ways (`require x v1.2` is an error, a malformed known keyword such as `require(` is an
  error, unknown lower-case verbs and their blocks are preserved); (2) `cflags`, `ldflags` and `pkg-config` lines of
  gocpp.mod are only PARSED: README §2.1 says they go through the allow-list, which lives in internal/build (it imports
  load), so the build step that uses them (6.5 or 6.9) must apply the checks to `Module.File()`; (3) module headers
  (README §2.11, `"github.com/ocornut/imgui/imgui.h"`) are not resolved: such an import is still a local header, a
  dotted first path element being ambiguous with local headers: needs the user's decision with the downloads
  question; `cxx path { }` recipes are only preserved (Block.Raw: the delimiting rule is a body line ending in `{` or
  `(` opens a nested block and a sole `}` or `)` closes it; the future recipe job owns the rule); (4) `gocpp mod vendor`
  is not implemented (6.12 or Phase 15 with downloads); (5) no version selection: only the main module's require and
  replace lines count, so a program must list every module it imports, directly or not; (6) the language server loads
  each package separately: a package of a replaced module does not see the main module's replaces (TODO.md Phase 11:
  the LSP must pass the main module folder to the loader); (7) hardening nit: the 16 MiB cap still allows large memory
  use for a hostile file of one-letter lines (about 190x the file size): a smaller cap or a line limit is cheap.
- 2026-10-17: Job 6.1b (module header imports, user decisions Q16/Q17) merged after two review rounds. Not a numbered step: it
  belongs to §2.11. Jobs 6.1c (downloads: `gocpp get`, `gocpp mod download|verify|vendor`, gocpp.sum, module cache) and 6.1d (read
  and build the `cxx` recipes) follow.
  Decisions: (1) own module AND required modules are candidates; longest module path at element boundaries wins; a first element
  with a dot is the marker (a top-level folder `v1.2/x.h` must be written `"./v1.2/x.h"`); (2) the generated `#include` is the
  ABSOLUTE file name (the emitted C++ differs between machines there, like `#line`); (3) the module root is an `-iquote` folder, NOT
  `-I`: `<x.h>` lookups of a library's own headers belong to the `include` folders of the 6.1d recipes (`-I`/`-isystem`); the package
  folder is searched before the module folder for a quoted include (documented); (4) a `replace` folder without gocpp.mod is accepted
  as module `req.Path` (a present gocpp.mod must declare the same path); a typo'd replace path to an existing folder is no longer
  caught; (5) hook for 6.1c: `Config.ModuleDir`/`moduleDir` (any returned dir is accepted, no gocpp.mod needed, `Module.cached`);
  hook for 6.1d: `moduleIncludeDirs(mod)`; (6) symlinks inside modules are followed by the loader (TODO.md: contain them for
  downloaded modules); (7) the gate for module headers moved to Phase 04 (matrix regenerated).
- 2026-10-17: Job 6.1c (module downloads) merged after two review rounds (a security-focused review; ADR-023). Decisions:
  (1) trust on first use: a build records a missing gocpp.sum line (printed once); no checksum database (a GOCPP_SUMDB is possible
  later); `-mod=readonly` (Go's meaning) never writes and refuses a missing line before any request; recommend it for CI;
  (2) sources are GitHub/GitLab/Codeberg archives, versions via git smart-HTTP info/refs, mirrors via GOCPP_MODURL/GOCPP_MODPROXY;
  (3) every symlink/hard link in an archive is rejected, so a library with links must be vendored in a fixed copy; unsafe Windows/
  macOS names are rejected on ALL platforms (same hash everywhere); (4) the language server never downloads or writes gocpp.sum;
  `gocpp check` and `build` may download; `ListModules` only looks in the cache; (5) a cache hit compares the ~h1 file with
  gocpp.sum and does not re-hash (`gocpp mod verify` does); (6) every test main sets GOCPP_MODCACHE=off and GOCPP_OFFLINE=1: keep it so
  for new test packages; (7) not done: file:// in GOCPP_MODPROXY, `mod tidy`, `gocpp clean -modcache` (6.12), a total cache size cap,
  `mod vendor` of two modules where one is a folder of the other. 6.1d (cxx recipes) is next: running.
- 2026-10-17: Job 6.1d (cxx recipes) merged after two review rounds (a security-focused review; ADR-024). Decisions:
  (1) the recipe language is the 13 statements in docs/spec/modules.md; the project's block overrides the library's own, no merging;
  (2) a recipe's `include`/`system-include`/`define`/pkg-config cflags apply to the DEPENDENT's compiles and probes, `cflags`/
  `cxxflags` are library-only; `include` is `-I` (the module root of 6.1b stays `-iquote`); (3) library compiles use `-w` and
  `-g -O0` until the Phase 06 build modes; (4) objects are shared only for modules in the module cache (the absolute source path is in
  the key); (5) OS conditions compare the exact GOOS string (`linux:` does not match android); (6) NOT built: shared recipe index,
  CMake fallback (open Q14 sub-questions); (7) gaps: check and the LSP do not look at library sources; only DIRECT importers carry
  CxxModules (6.2 must take the union over packages); no `needs` statement; the module-wide cflags/pkg-config/ldflags of a
  dependency's gocpp.mod are still not applied (allow-list question); go-to-definition for `<x.h>` through a recipe include is not
  resolved; a symlink inside an `-L` folder that leaves the module is not checked (downloads have no symlinks).
- 2026-10-18: Jobs 6.2 and 6.3 (per-package codegen, header layout) merged after two review rounds (ADR-025). Decisions: (1) a package
  = `gpkg/<import path>/pkg.hpp` + one `<file>.gp.cpp` per file with functions + `pkg_desc.cpp` + `pkg_init.cpp`; main's pkg_init.cpp has
  `gp_init_all` and `main` (namespace gpkg_main whatever the import path); (2) descriptors are `extern` in pkg.hpp and defined in
  pkg_desc.cpp (not inline in the header); forwarders are decided on the struct's OWNER, declared in the struct, defined in the owner's
  pkg_desc.cpp; (3) unnamed struct/interface types: guarded per-header definitions in the global namespace (not gpkg_anon.hpp); the hash
  is 64 bits (collision handling not done); (4) generated files live in `gen/<hash of main folder, path, file names, target>` of the
  user cache under an advisory flock (`gen/<hash>.lock`) held from the write step to the end of compile; fallback: a private folder
  `gen/<hash>-<pid>-<n>` (removed on success; Windows uses it until LockFileEx is done); `trimGen`: >100 folders and untouched for a
  week; (5) generated files are exempt from the object cache's 2 s racy window; (6) the cxx libraries are the UNION over packages
  (closes the 6.1d gap); (7) `lower.Package`/`cppgen.EmitPackage` are TEST-ONLY (~100 goldens exercise a layout nobody ships); TODO:
  route the goldens through the program path; (8) 6.4 partial: `gp_init_all` order done; NOT `gp::lazy<T>` and lazy c-archive/c-shared
  init; (9) single-file programs compile 2 units (+0.3 s clean); `gocpp emit` prints all files after `//// path` lines; (10) the
  'more than one package' gate stays until version.Phase reaches 6 (5.12 lifts gates; tests use `// phase: 06` and GOCPP_TEST_PHASE=06);
  (11) macros of a package's C headers leak into dependents' files through pkg.hpp (documented); import paths differing only by case
  collide on case-insensitive file systems; (12) stub std packages = any function without a body: Phase 09 needs a real marker;
  (13) mutant gap: dropping the hash from `unitName` survives (two file names differing only in unsafe characters would collide):
  add a fixture. NEXT: 5.7 (`errors`) and 5.11 (`fmt` on any) can start now (multi-package exists); examples/multi-pkg needs them.
