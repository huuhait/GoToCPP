# Phase 14: Windows · STATE

**Status:** `[~]` in progress (14.1 merged 2026-10-20; verified on a real Windows 11 PC over ssh) (the runtime is portable from Phase 02; Windows test builds start in Phase 06)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] 14.1 Windows OS layer in the runtime (console UTF-8, UTF-16 args/env, threads, stack-overflow handler, `_exit`, stack traces) + unit tests under Wine
- [x] 14.2 Build driver: MinGW-w64 detection + the pinned llvm-mingw download on a Windows host (Phase 06 §2.10), `.exe`/`.dll` + import libs + `dllexport`, `-static` linking, response files, spaces/drive letters, cache location, CRLF
- [x] 14.3 Standard library for Windows (merged 2026-10-19; ADR-076: Go's own _windows files over four natives)
  - [ ] `portstd` with `GOOS=windows`
  - [ ] native `internal/syscall/windows` on `<windows.h>`
  - [ ] `os`, `path/filepath`, `time`, `os/exec`, `os/signal` + tests
- [x] 14.4 Windows interop: `WIN32_LEAN_AND_MEAN`/`NOMINMAX`/`UNICODE` defaults, macro collisions, `C.wstr`/`C.GoStringW`, Windows header zoo + SDL2/raylib/sqlite3/zlib for MinGW-w64 (merged 2026-10-19; ADR-082)
- [ ] 14.5 Language server + VS Code on Windows (paths, URIs, file watching, case-insensitive names) + Phase 11 budgets on Windows
- [ ] 14.6 Debugging (gdb/lldb, VS Code launch template) + typed-interop probes with the Windows target
- [ ] 14.7 Test infrastructure: Wine CI job for the whole suite (`GOOS=windows` Go as reference) + real-Windows release checklist
- [ ] 14.8 Docs: install on Windows, build for Windows from Linux, Windows interop cookbook

## Exit criteria

- [ ] Phase 00 target programs build and run on Windows (Wine in CI + a real Windows machine)
- [ ] Run, error and differential suites pass for windows/amd64 under Wine
- [ ] `gocpp.exe` works natively; VS Code edits, builds and debugs on Windows
- [ ] Win32 example (`MessageBoxW` + a small window) and raylib/SDL example run on Windows
- [ ] Phase 11 editor budgets hold on Windows

## Decisions / notes

- 2026-10-13 (your decision, Q8): Linux and Windows are the targets, Linux first, Windows before
  v1.0; macOS is not a target. Toolchain: MinGW-w64 (clang or gcc) first, cross-compiled from
  NixOS and tested under Wine; the MSVC ABI (`clang-cl`) later, for C++ libraries built with
  Visual Studio. This phase was added before hardening, which became Phase 15.
- 2026-10-13 (dev-shell review): Wine runs follow the Phase 06 §2.10 rules; Linux CI tests the
  msvcrt-flavored MinGW-w64, so UCRT (llvm-mingw, MSYS2) is checked only on real Windows.
