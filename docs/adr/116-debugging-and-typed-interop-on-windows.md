# ADR-116: Debugging and typed interop on Windows: the script travels in the program, the probe is a clang with the target

Status: accepted (2026-10-20), step 14.6. Builds [ADR-066](066-debugger-support.md) (`gocpp debug`, the gdb script), [ADR-053](053-windows-os-layer-on-createthread.md) (the Windows OS layer),
[ADR-060](060-typed-interop-probes.md) and [ADR-088](088-typed-interop-caches-every-compiler-answer-per-reference-under-the-headers-contents.md) (the typed probes and their cache),
[ADR-082](082-windows-interop-defaults-and-wide-strings.md) (the Win32 defaults) and [ADR-108](108-typed-interop-becomes-the-default-when-three-measurements-pass-and-the-clang-policy-is-a-matrix.md) (the third measurement: a Windows host).

## Context

Two things were left for Windows by the earlier steps of Phase 14.

1. `gocpp debug` ran on a Windows host only by luck. The gdb script of ADR-066 lives in a `.debug_gdb_scripts` section that only the ELF layer wrote, so a program for Windows had no printers
   unless the user ran `source` by hand; the debugger was looked up in PATH only; the safe path for auto-loading was built for Linux (a `:`-separated list, backslashes in paths); the script
   did not know the Windows data model (`long` has 32 bits, so `int` showed as `int64`) and could not find the goroutine of a thread (the Windows layer keeps it in a `TlsAlloc` index, not in a
   `thread_local`); and there was no VS Code configuration.
2. The typed probes (Phase 12) ran the host's compiler without a target. On a Windows host `auto` was opaque by a rule; for a Windows target built on Linux the probes were restricted to the
   headers of the C++ library, because the dev-shell clang has no MinGW-w64 headers (TODO.md, 14.4 low) and cannot parse `<windows.h>`.

## Decision

**The script travels in the program, for PE too.** `runtime/src/os/windows/debug_scripts.cpp` writes the same section as the ELF layer, from one shared `runtime/src/os/gocpp_gdb.inc` (moved
from `src/os/linux/`). COFF has no `.pushsection`: the file uses `.section .debug_gdb_scripts, "dr"` and returns to `.text`. Since an archive member is linked only when something refers to a symbol
in it, `rt.cpp` takes the address of the member's symbol; the same reference fixes a regression found on the way: on plain dev the Linux section had been missing from programs that link
`libgocpprt.a` (`TestDebugGDB` failed with "Undefined command: gp").

**`gocpp debug` on Windows.** The debugger is found by `toolchain.Config.FindTool` before anything is built (PATH, then the folders an install uses: MSYS2, Chocolatey, Scoop, WinGet's WinLibs,
the usual MinGW folders, then the pinned llvm-mingw), so a missing gdb costs no build and the error says what to install. Every path handed to the debugger is written with forward slashes (gdb
takes a backslash as an escape; both debuggers read slashes); the safe-path list separator is `;` on Windows, so the colon of a drive letter is not a problem, and the by-hand fallback is used
only when the path holds the separator. A program for another system than the host's cannot run under the host's debugger: that is an error, except with `-print`. The script knows LLP64
(`apply_data_model` maps `long long` to `int`, `int64`, `uint`, `uint64`, `uintptr`), and finds the goroutine of a thread from the TEB: `$_tlb` plus `TlsSlots[index]` (or the expansion array),
with the index in `gp::os::debug_tls_slot`, a global that `thread.cpp` sets. `gocpp debug -vscode` prints a `launch.json` for the target: cppdbg with gdb (`miDebuggerPath` is the found gdb on this
machine, else `gdb`/`gdb.exe`; `miDebuggerArgs` has the safe path; `sourceFileMap` from `-map`) or lldb-dap (`initCommands` import `.vscode/gocpp_lldb.py`, which the command writes). The guide
shows the same text, and a test compares them. `$gp_blocked()` read a counter that step 8.11 had replaced by `sched::counts`; it is fixed for both systems.

**The typed probe is a clang with the target.** `probe.FindClangFor(goos, goarch)` returns this machine's clang for its own target. For windows/amd64 it returns, in this order, `GOCPP_CLANG`,
`x86_64-w64-mingw32-clang++`, the build's compiler for the target when it is clang (which includes the pinned llvm-mingw), and `clang++` in PATH, and marks it with the target (`CXX.GOOS`, `GOARCH`).
`build.ProbeTargetFlags(cxx)` gives the flags that go with a marked compiler: `--target=` for a clang that does not name its target (`clang++-21` is not a target name), `-D_WIN32_WINNT=0x0A00`,
and, for such a clang on a system other than Windows, `-fno-file-reproducible` (clang writes `__FILE__` with backslashes for a Windows target, and MinGW-w64's mcfgthread headers include
themselves by `__FILE__`) and the include folders of the MinGW-w64 g++ that the cross build uses (`-nostdlibinc -nostdinc++ -isystem ...`, without GCC's own intrinsic headers, which clang replaces).
The kind probe, the macro probe, the typed probes and `gocpp doc` pass these flags before the package's flags; `gocpp doc` also passes the Win32 defaults for windows (`build.Win32DefaultsFor`).
The target and its flags are part of the keys of the kind-probe cache and (through `CXX.GOOS`/`GOARCH`) of the typed probe cache. `typedinterop.ProbesTarget` says which targets the probes answer
for (this machine's, and windows/amd64); the others stay restricted to the C++ library's headers. The rule "`auto` is opaque on a Windows host" is removed: `auto` is typed when a clang is found.

## Consequences

- A Windows program that imports `<windows.h>` can be built in typed mode from Linux and on a Windows host, with LLP64 types (`DWORD` is `unsigned long`).
- On Linux the target needs the MinGW-w64 g++ that the Windows cross build needs anyway; with only a clang and no such g++, the probe fails to find headers and the build falls back to opaque (`auto`)
  or reports it (`typed`).
- The key layout of the kind-probe cache changed (it now holds the target and its flags), so the first run after upgrading is cold for every target, the host's too. Nothing stale is read.
- `-fno-file-reproducible` needs clang 14 or later; gocpp's minimum is clang 17 (`build.MinClang`, enforced by `LookCXX`), so the flag is always known.
- The folders that `FindTool` (and the compiler search before it) looks in include ones a user can create, such as `C:\mingw64`, `C:\w64devkit` and `C:\winlibs`. The trust model is the compiler search's:
  PATH first, then known install folders; a debugger found there is run as the user, like the compiler. `gocpp debug -x` prints which one it started.
- `targetFlags` treats only a program name that starts with an architecture (`x86_64-`, `aarch64-`, ...) as naming its target, so a GOCPP_CXX of `clang++-21` for another target now gets `--target`.
- The VS Code files are generated, not checked in an editor: nobody has launched them in VS Code (no VS Code on the PC or in the dev shell).

## Not done, not measured

- lldb and lldb-dap on Windows: the PC has no lldb; the lldb script is only text-checked.
- A launch from VS Code on Windows or Linux.
- The interop corpus on Windows (ADR-108's third measurement): the probe, a typed build, a typed run and `gocpp doc <windows.h>` ran on the PC with the pinned llvm-mingw; MSYS2's and WinLibs'
  clangs were not tried (the PC has no clang of its own).
- Stepping (`next`, `step`) through a long session on Windows.
