# Phase 14: Windows

> Make everything that works on Linux work on Windows too: build `.exe` and `.dll` files, pass
> the whole test suite, use Windows headers (`<windows.h>`), edit and debug. Linux stays the first
> platform, but the runtime has been portable since Phase 02 and Windows test builds have run
> under Wine since Phase 06, so this phase **finishes** the port instead of starting it.

| | |
|---|---|
| Depends on | Phase 06 (cross-compilation, first Windows builds), Phase 08 (thread layer), Phase 09 (stdlib, `portstd` per target OS); Phases 11–13 for the editor, typed-interop and debugger parts |
| Unlocks | Phase 15 (v1.0 ships for Linux and Windows) |
| Milestone | **M8: Windows works** |
| Size | medium-large (about 4–6 weeks) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Your decision (2026-10-13): **Linux and Windows** are the targets, Linux first, and Windows must
work before v1.0. macOS is not a target.

Most of gocpp doesn't care about the platform. The compiler and the language server are Go
programs, and Go builds `gocpp.exe` for Windows with one command. The generated code is standard
C++20. What's left is small and known in advance:
- the runtime's OS layer (Phase 02) and thread layer (Phase 08);
- the build driver: toolchain, linking, Windows file and command-line limits;
- the standard library's system calls (Go's own `_windows.go` files already exist for these);
- Windows headers in interop;
- debugging, and file paths in the editor.

**Toolchain: MinGW-w64 first** (clang or gcc for `x86_64-w64-windows-gnu`):
- the same compiler drivers and flags as on Linux (`-fwrapv`, `-std=c++20`), DWARF debug info
  (so the Phase 13 gdb/lldb printers keep working), and pthreads through winpthreads (so the
  Phase 08 thread code mostly works unchanged);
- it cross-compiles from your NixOS machine (Nix's MinGW-w64 cross toolchain), so tests run
  under **Wine** in CI without a Windows machine;
- on Windows itself it is one download (llvm-mingw, or MSYS2's mingw-w64 packages). No Visual
  Studio needed.

**The MSVC ABI comes later**, with `clang-cl` for `x86_64-pc-windows-msvc`. It is only needed to
link prebuilt C++ libraries that were built with Visual Studio; C libraries already work with
MinGW, and C++ sources are compiled by gocpp's own toolchain anyway. `clang-cl` accepts `-fwrapv`
(checked in the audit). MSVC's own compiler has no equivalent, so it isn't supported.

## 2. How it works

### 2.1 Runtime: the Windows OS layer

The runtime reaches the operating system only through `runtime/src/os/` (Phase 02), which also
holds the `gp::os` thread layer (Phase 08). This phase adds the Windows versions:

| Job | Linux | Windows |
|---|---|---|
| write to stdout/stderr | `write(2)` | `WriteFile`; when the handle is a console, `WriteConsoleW` with UTF-8 → UTF-16, so non-ASCII text shows correctly |
| program arguments | `argc`/`argv` | `GetCommandLineW` + `CommandLineToArgvW`, converted to UTF-8 (Go does the same) |
| environment | `environ` | `GetEnvironmentStringsW`, converted to UTF-8 |
| threads with a stack size | pthreads | winpthreads (same code, linked with `-pthread`); note that `std::thread` in nixpkgs' MinGW GCC sits on its own thread library, mcfgthread |
| stack overflow message | guard page + `SIGSEGV` on `sigaltstack` | `SetThreadStackGuarantee` + a vectored exception handler for `EXCEPTION_STACK_OVERFLOW` → `fatal error: stack overflow`, exit 2 |
| exit after flushing | `_exit` | `_exit` from the MinGW C runtime |
| stack traces in panics | `backtrace()` or libbacktrace (`<stacktrace>` is C++23, not available under `-std=c++20`, Phase 03 §2.9) | `CaptureStackBackTrace` + DWARF line lookup (libbacktrace reads MinGW's DWARF too) |
| blocking in `gp::waiter` | condition variables / futex | condition variables (C++20 `std::atomic::wait` works on both) |
| clocks | `std::chrono` | `std::chrono` (QueryPerformanceCounter underneath) |

### 2.2 Build driver

- **Toolchain detection** for MinGW-w64 (`x86_64-w64-mingw32-clang++`, `x86_64-w64-mingw32-g++`,
  llvm-mingw on Windows); `gocpp env` shows what it found.
- **Outputs**: `.exe`; `c-shared` gives a `.dll` plus an import library (`.dll.a`), and `//export`
  functions are marked `__declspec(dllexport)`; `c-archive` gives a `.a`.
- Programs are linked with **`-static`**, so an `.exe` runs without extra DLLs. Linking only
  `-static-libgcc -static-libstdc++` is not enough with nixpkgs' MinGW GCC: the `.exe` then still
  needs `libmcfgthread-2.dll` and fails to start (seen while building the dev shell). Phase 06
  already does this for the first Windows builds.
- **Windows limits**: a command line can be at most 32,767 characters, so long compile and link
  commands go through **response files** (`@file`); paths can contain spaces and drive letters;
  the cache lives in `%LocalAppData%\gocpp`.
- **CRLF** line endings in `.gp` files work (the Go scanner already handles them; raw strings drop
  `\r` as in Go), and `gocpp fmt` writes LF like gofmt.
- `pkg-config` works under MSYS2; without it, `//gocpp:cflags`/`//gocpp:ldflags` and
  `gocpp.mod` flags do the job.

### 2.3 Standard library

- `tools/portstd` runs again with `GOOS=windows` (Phase 09 prepared it), so Go's own Windows code
  comes along: `os` (`file_windows.go`, errors), `path/filepath` (`\` and drive letters), `time`
  (time zones from the registry), `os/exec`, `os/signal` (Ctrl-C).
- Go's Windows code calls the Win32 API through `syscall` (loading DLLs, `syscall.SyscallN`). gocpp
  replaces that one layer with a native `internal/syscall/windows` built on the `<windows.h>`
  header package, so the code above it ports mechanically.
- Linux-only by nature: `-race`, because ThreadSanitizer doesn't support Windows. AddressSanitizer
  and UBSan work with clang's MinGW target.

### 2.4 Interop with Windows headers

- `<windows.h>` is a header package like any other: `windows.MessageBoxW(nil, …)`,
  `windows.GetLastError()`.
- gocpp defines `WIN32_LEAN_AND_MEAN` and `NOMINMAX` by default (`//gocpp:no-win32-defaults`
  turns them off), so
  the `min`/`max` macros can't break C++ code. Phase 04's macro protection (`#undef` /
  `push_macro`) handles the other collisions (`ERROR`, `DELETE`, `interface` from `<objbase.h>`).
- `UNICODE` is defined by default, as Go's own Windows code uses the `W` functions. So
  `windows.MessageBox` (a macro) means `MessageBoxW`.
- Win32 `W` functions take UTF-16 text. `C.wstr(s)` gives a `const wchar_t*` for the statement
  (a holder, like Phase 04's `C.cstr`), and `C.GoStringW(p)` converts back.
- x86-64 Windows has a single calling convention, so callbacks work as in Phase 04.

### 2.5 Editor, debugger, typed interop

- `gocpp.exe`, the language server and the VS Code extension run natively on Windows.
- **Language server**: Windows paths ↔ `file:///C:/…` URIs (drive-letter case, percent
  encoding), case-insensitive file names in the snapshot cache, file watching. Phase 11's speed
  budgets are measured on Windows too.
- **Debugging**: MinGW builds carry DWARF, so `gocpp debug` uses gdb (MSYS2) or lldb
  (llvm-mingw) with the Phase 13 printers, and VS Code works through CodeLLDB. The Visual Studio
  debugger (PDB + natvis) comes with the MSVC ABI later.
- **Typed interop** (Phase 12): probes run with the Windows target and the MinGW-w64 headers.

### 2.6 Testing Windows from Linux

- **CI on Linux**: every run test, error test, differential test and example is cross-built for
  Windows and run under Wine (console programs; GUI examples run their smoke mode), with the
  Wine rules from Phase 06 §2.10 (own prefix created up front, `wineserver -p`,
  `WINEDEBUG=-all`, no `winemenubuilder`). These builds use the dev shell's msvcrt-flavored
  MinGW-w64, so the C runtime that users get from llvm-mingw or MSYS2 (UCRT) is only tested by
  the real-Windows runs below. Expected
  outputs of OS-dependent behavior (paths, error texts) come from real Go built with
  `GOOS=windows`, also run under Wine.
- **Real Windows** (a machine or VM; there's no GitHub CI yet, decision Q9): the same suite and
  the editor benchmarks before every release, and weekly while this phase runs.

## 3. Steps

1. **14.1 Windows OS layer** in the runtime (§2.1) + its unit tests under Wine.
2. **14.2 Build driver** (§2.2): MinGW-w64 detection, `.exe`/`.dll` + import libraries +
   `dllexport`, `-static` linking, response files, spaces and drive letters, cache location,
   CRLF sources.
3. **14.3 Standard library** for Windows (§2.3): `portstd` with `GOOS=windows`, a native
   `internal/syscall/windows` on `<windows.h>`, then `os`, `path/filepath`, `time`, `os/exec`,
   `os/signal`, with tests.
4. **14.4 Windows interop** (§2.4): `WIN32_LEAN_AND_MEAN`/`NOMINMAX`/`UNICODE` defaults, the
   macro-collision cases, `C.wstr`/`C.GoStringW`, a Windows header zoo (`windows.h`, `winsock2.h`,
   `ws2tcpip.h`, `shellapi.h`) plus SDL2, raylib, sqlite3 and zlib built for MinGW-w64.
5. **14.5 Language server and VS Code on Windows** (paths, URIs, file watching, case-insensitive
   names); the Phase 11 budgets measured on Windows.
6. **14.6 Debugging and typed interop on Windows**: `gocpp debug` with gdb/lldb, the VS Code
   launch template, Phase 12 probes with the Windows target.
7. **14.7 Test infrastructure**: the Wine CI job for the whole suite (with `GOOS=windows` Go as the
   reference for OS-dependent output) and the real-Windows release checklist.
8. **14.8 Docs**: installing gocpp on Windows (llvm-mingw or MSYS2), building for Windows from
   Linux, and a Windows interop cookbook (`MessageBoxW`, a small Win32 window, sockets).

## 4. Deliverables

- The Windows OS layer, the Windows build driver, the standard library for Windows, Windows interop
  defaults and helpers
- A Wine CI job for the whole suite and a real-Windows release checklist
- Windows docs and cookbook

## 5. Exit criteria

- The Phase 00 target programs build and run on Windows, under Wine in CI and on a real Windows
  machine.
- The run, error and differential test suites pass for windows/amd64 under Wine.
- `gocpp.exe` works natively on Windows (`build`, `run`, `test`, `fmt`, `lsp`), and VS Code edits,
  builds and debugs a gocpp program there.
- A Win32 example (a `MessageBoxW` call and a small window) and the raylib/SDL example run on
  Windows.
- Phase 11's editor budgets hold on Windows.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Wine behaves differently from real Windows | Real-Windows runs before every release; Wine is only the fast feedback loop |
| `<windows.h>` macros break code | `WIN32_LEAN_AND_MEAN` + `NOMINMAX` defaults, Phase 04 macro protection, the Windows header zoo |
| Users need C++ libraries built with Visual Studio | MinGW covers C libraries and source builds; the MSVC ABI (`clang-cl`) is the planned follow-up |
| No ThreadSanitizer on Windows | `-race` stays Linux-only; the concurrency code is shared, so the Linux TSan runs cover it |
| Path and text-encoding bugs (UTF-16, drive letters) | Go's own Windows code is ported, not rewritten; tests with non-ASCII names and paths with spaces |
