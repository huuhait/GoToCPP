# Phase 06: Packages, Modules & Build System · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [ ] 6.1 `gocpp.mod` (modfile lax + `replace`/gocpp lines read from the syntax tree), module root, `replace`, `vendor/`
- [ ] 6.2 Per-package codegen (`pkg.hpp`, one TU per `.gp`, `pkg_init.cpp`, flat escaped namespaces, stable folders)
- [ ] 6.3 Header layout (gocpp headers first, public flags, fully qualified struct field types, exported constants, shared anon-type header)
- [ ] 6.4 Cross-package init: Go 1.21+ global order, `gp_init_all()`, `constinit` variables, `gp::lazy<T>`
- [ ] 6.5 Action graph with content hashing + `-MD` header deps + macro lists + public flags + wrapper environment
- [ ] 6.6 Cache (objects, generated files, write-if-changed, API hash = export data + `pkg.hpp`, package summaries, `clean -cache`)
- [ ] 6.7 Parallel executor (`-p`), `-x`, `-work`, error aggregation
- [ ] 6.8 Runtime prebuild + precompiled header
- [ ] 6.9 Link step (ordering, dedup, system libs, LTO with matching archiver)
- [ ] 6.10 Build modes
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

