# ADR-021: The platforms are Linux and Windows (MinGW-w64 first)

Status: accepted (2026-10-13)

## Context

The user's targets are Linux and Windows, Linux first, and macOS is not a target
([Q8](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)). Windows needs a C++
toolchain that accepts gocpp's flags (`-fwrapv`, [ADR-006](006-numbers-and-checks.md)), and the
runtime must not call POSIX functions all over its code.

## Decision

- **Targets: Linux and Windows, x86-64 first.** Linux comes first. Windows test builds start in
  Phase 06 (cross-compiled, run under Wine), the full port is Phase 14, and both work before
  v1.0.
- **Windows uses MinGW-w64 first** (clang or gcc for `x86_64-w64-windows-gnu`): the same
  compiler drivers and flags as on Linux, DWARF debug info, and pthreads through winpthreads. It
  cross-compiles from NixOS, and on Windows it is one download (llvm-mingw or MSYS2), with no
  Visual Studio. Programs are linked with `-static`, so an `.exe` needs no extra DLLs.
- **The MSVC ABI comes later**, with `clang-cl`. It is only needed to link C++ libraries that
  were built with Visual Studio.
- **The runtime reaches the OS through one layer**: only through `runtime/src/os/`, which also
  holds the Phase 08 thread layer `gp::os`. Linux gets its version in Phase 02 (threads in
  Phase 08), Windows in Phase 14. A CI check fails if the runtime uses a POSIX header anywhere
  else. (The standard library's own per-OS files, such as `file_windows.gp`, are the other
  allowed place.)

| Job | Linux | Windows |
|---|---|---|
| write to stdout | `write(2)` | `WriteFile`; `WriteConsoleW` (UTF-8 → UTF-16) on a console |
| threads with a stack size | pthreads | winpthreads (same code, linked with `-pthread`) |
| stack overflow message | guard page + `SIGSEGV` on `sigaltstack` | `SetThreadStackGuarantee` + a vectored exception handler |
| stack traces in panics | `backtrace()` or libbacktrace | `CaptureStackBackTrace` + DWARF line lookup |

## Alternatives

- **macOS** (rejected): not a target. The code stays portable (standard C++, one OS layer), so
  a port is possible later.
- **MSVC's own compiler** (rejected): it has nothing like `-fwrapv`.

## Consequences

Good: Windows breakage shows up early (Wine tests from Phase 06); the standard library reuses
Go's own `_windows.go` files; MinGW builds carry DWARF, so the same gdb/lldb printers work on
Windows too.

Bad:
- `-race` is Linux-only: ThreadSanitizer doesn't support Windows. ASan and UBSan work with
  clang's MinGW target.
- Windows needs extra care: command lines of at most 32,767 characters (response files), UTF-16
  text (`C.wstr`), `<windows.h>` macros, drive letters and spaces in paths.
  `WIN32_LEAN_AND_MEAN`, `NOMINMAX` and `UNICODE` are on by default;
  `//gocpp:no-win32-defaults` turns off the first two.
- libc++ (used by llvm-mingw) has no `std::atomic<std::shared_ptr>`, so `atomic.Pointer[T]` uses
  a small lock (`gp::atomic_ptr<T>`).
- Wine is not real Windows, and the Linux CI uses the dev shell's MinGW-w64, which targets the
  older `msvcrt.dll` C runtime, while users' toolchains (llvm-mingw, MSYS2 UCRT64) target UCRT.
  So the suite also runs on a real Windows machine before every release.
- No C++ libraries built with Visual Studio until the MSVC ABI work (after 1.0). Linux arm64
  comes in Phase 15, Windows arm64 after 1.0.

## Evidence

- Audit: `clang-cl` accepts `-fwrapv`, so the later MSVC-ABI route keeps Go's overflow rule.
- Seen while building the dev shell: with only `-static-libgcc -static-libstdc++`, an `.exe`
  from nixpkgs' MinGW GCC still needs `libmcfgthread-2.dll` and fails to start. Hence `-static`.

See: [Phase 14 §1](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#1-idea), [§2.1](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#21-runtime-the-windows-os-layer), [§2.2](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#22-build-driver), [§2.4](../../plans/2026-10-13-gocpp/phase-14-windows/README.md#24-interop-with-windows-headers), [Phase 06 §2.10](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#210-cross-compilation-and-the-first-windows-builds), [Phase 02 §2.6](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#26-runtime-v0-runtimeincludegocpprthpp), [Phase 08 §1](../../plans/2026-10-13-gocpp/phase-08-concurrency/README.md#1-idea), [Phase 15 §2.3](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#23-platforms).
