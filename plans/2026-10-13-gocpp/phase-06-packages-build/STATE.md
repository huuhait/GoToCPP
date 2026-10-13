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
- [ ] 6.13 Cross-compilation: windows/amd64 via MinGW-w64 (`.exe` linked with `-static`) + Wine CI job; linux/arm64 best effort
- [ ] 6.14 Tests: multi-package fixtures, init order vs Go, incremental-build assertions (incl. constants), build-mode consumers (incl. two c-shared libs in one process)

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
