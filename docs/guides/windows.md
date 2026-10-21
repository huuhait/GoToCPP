# gocpp on Windows

> `gocpp.exe` runs on Windows 10 or later (x86-64), builds `.exe` and `.dll` files there and edits
> in VS Code. The C++ compiler is a MinGW-w64 one, which gocpp downloads once if you have none. The
> other Windows pages: [building for Windows from Linux](windows-cross.md) and
> [calling the Win32 API](windows-interop.md). The rules behind this page are in the
> [build spec](../spec/build.md#gocppexe-on-a-windows-host) and in
> [ADR-081](../adr/081-windows-host-build-driver.md).

Everything on this page was tried on a real Windows 11 PC (the test suites of Phase 14 run there
over `ssh`), with two MinGW-w64 compilers: WinLibs' GCC 16.1 and the pinned llvm-mingw. Where a
sentence describes something that no run has seen (it is read from the code), it says so. What does
not work yet is in [the last section](#what-does-not-work-yet).

## Install gocpp

You need `gocpp.exe` and nothing else (the C++ runtime and the standard library are inside it):

```powershell
go install github.com/huuhait/gocpp/cmd/gocpp@latest
gocpp version
```

`go install` needs Go 1.25 or newer and writes `gocpp.exe` to `%GOBIN%` (by default
`%USERPROFILE%\go\bin`, which the Go installer already puts in `PATH`). The other way is the release
archive, `gocpp_<version>_windows_amd64.zip`: unzip it and put the folder that holds `gocpp.exe` in
`PATH` ([installing gocpp](packaging.md) describes the archives; releases themselves come with
Phase 15, so today `go install` from the repository is the way in). Open a new terminal and check that
`gocpp version` prints the version, the Go version and `windows/amd64`:

```text
gocpp <version> (phase NN) go1.25.14 windows/amd64
runtime <hash>
```

`gocpp.exe` is one static file: no installer, no registry entry, no service. It keeps its files in
`%LocalAppData%\gocpp` (the cache; `gocpp env GOCPPCACHE` prints the folder) and uses `%TEMP%` for the
folders of a build, which it removes afterwards (it keeps one only to report an internal error).

## The C++ compiler

The C++ that gocpp writes is compiled with a **MinGW-w64** compiler: GCC 13 or newer, or clang 17 or
newer, for `x86_64-w64-mingw32` (clang says `x86_64-w64-windows-gnu`). Visual Studio is not needed
and not used. `gocpp build` and `gocpp run` look for one in this order and use the first they find:

1. `GOCPP_CXX`, then `CXX`: a compiler you name yourself.
2. `x86_64-w64-mingw32-clang++` or `x86_64-w64-mingw32-g++` in `PATH` (the names that llvm-mingw and the cross packages of Linux
   distributions use; WinLibs has them too).
3. a plain `clang++` or `g++` in `PATH`, **if it builds MinGW programs**: gocpp asks it with
   `-dumpmachine`. Visual Studio's own LLVM (`clang++` for `windows-msvc`), the `g++` of MSYS2's `msys`
   environment and Cygwin's `g++` build programs that need other runtimes, and are skipped.
4. the install folders of MSYS2 (`ucrt64`, `clang64`, `mingw64` below `C:\msys64` or `MSYS2_ROOT`),
   Chocolatey, Scoop, WinGet's WinLibs, and `C:\mingw64`, `C:\w64devkit`: so a compiler that its
   installer did not put in `PATH` is still found. A compiler in `PATH` always wins over these.
5. **gocpp's own copy of llvm-mingw**. If nothing above was found, `gocpp build` and `gocpp run`
   download it once: a zip of about 190 MB from the llvm-mingw release page, into
   `%LocalAppData%\gocpp\toolchain\llvm-mingw-<release>-windows-amd64\`. The size and the SHA-256
   of the archive are inside `gocpp.exe`, and are checked before anything is unpacked; the download
   goes over https only. So on a machine with nothing installed, `gocpp run hello.gp` downloads one file
   and works. `gocpp check` and the editor never download, and neither does anything with
   `GOCPP_OFFLINE=1` (a toolchain that is already downloaded is still used). `gocpp clean -toolchain`
   removes the download.

`gocpp env GOCPPCXX GOCPPCXXSOURCE` says which compiler a build would use and why (`PATH`, `GOCPP_CXX`, a
folder name, the download):

```text
C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\...\mingw64\bin\x86_64-w64-mingw32-g++.exe
PATH
```

`GOCPP_TOOLCHAIN` changes the rule: `auto` (the default: the list above), `local` (never download; for
offline machines and CI) and `pinned` (always gocpp's llvm-mingw, so that every machine builds with
the same compiler). When nothing is found and gocpp may not download, it says what it looked at and how to
install one:

```text
gocpp build: no C++ compiler for windows/amd64: no MinGW-w64 g++ or clang++ is in PATH, and none is in the
folders of MSYS2, Chocolatey, Scoop or WinGet's WinLibs (GOCPP_TOOLCHAIN=local never downloads one; install MinGW-w64
(WinLibs: winget install BrechtSanders.WinLibs.POSIX.UCRT; MSYS2: pacman -S mingw-w64-ucrt-x86_64-gcc) or llvm-mingw,
or set GOCPP_CXX)
```

The download of llvm-mingw exists for three kinds of host (`internal/toolchain.Archives`): Windows x86-64 (the zip above) and, for `-os windows` builds, Linux x86-64
and arm64 ([Windows from Linux](windows-cross.md)).

Both C runtimes work: msvcrt (tried with WinLibs' `MSVCRT` build, GCC 16.1) and UCRT (tried with llvm-mingw, the download, whose
hello world builds and runs on the PC). MSYS2's `ucrt64` and WinLibs' `UCRT` build were not tried on their own. UCRT is the one to choose when you install your
own, since it is the C runtime of the download. A program is linked statically (`-static`), so the `.exe` needs no DLL of the compiler next to it.
Only 64-bit x86 Windows is a target: no 32-bit, no arm64, and no Visual Studio toolchain (`cl.exe`,
`clang-cl`, the MSVC ABI, PDB files).

## A first program

Save this as `hello.gp` in any folder:

<!-- program: 02 id=winhello go -->
```go
package main

import "fmt"

func main() {
	fmt.Println("hello, Windows")
}
```

<!-- output -->
```text
hello, Windows
```

```sh
gocpp run hello.gp
gocpp build hello.gp
```

`gocpp run` builds and starts the program; `gocpp build` leaves `hello.exe` next to the source (`-o prog`
writes `prog.exe`; a name with an extension keeps it). The first build compiles gocpp's C++ runtime and
caches it, which took 10 to 15 seconds on the test PC (16 threads); the next `gocpp run` of the same file took about a
second. The `.exe` is about 12 MB (it is static and carries debug information) and starts on any Windows 10 or later.

Writing `gocpp build -x` shows the commands. `gocpp emit hello.gp` prints the C++ that was
generated, as on Linux.

## Folders, spaces and non-ASCII names

A path with **spaces** works everywhere. A path with letters outside your ANSI code page (a user called
`Łukasz` on a Western system, a folder `Wörk Dir 日本語`) needs care, because MinGW-w64's compilers read their command line in the
ANSI code page; gocpp takes the care for you:

- A project folder or source file with such a name works: `gocpp run .\höllo.gp` in `Wörk Dir 日本語` printed
  `héllo 世界` correctly and `gocpp build -o prog` wrote `prog.exe` there. gocpp's own file access uses the wide
  Windows API, and it hands the compilers short names (`WORKDI~1`) or ASCII names of its own for what they must open. A compiler that reads
  Unicode command lines (GCC 14 and later, clang) gets the real names.
- The folders gocpp **writes** its files in are checked at the start: if `LOCALAPPDATA` (the cache), `TEMP` or
  `TMP` has a character outside the ANSI code page, or `TEMP`/`TMP` is longer than 200 characters, gocpp uses the
  8.3 short name of the same folder for the compilers. A name that is already a legal 8.3 name (`Łukasz`) has no short
  name, so those users get a **private folder** `%ProgramData%\gocpp-paths-<SID>\...`. It is made for you alone (the access list
  names your account, SYSTEM and Administrators, nobody else), and an existing folder of that name is used only if you own it
  and no one else has access to it; otherwise gocpp refuses it and says so. `gocpp env GOCPPCACHE` shows the folder in use.
  The program that `gocpp run` starts gets your original `TEMP`, `TMP` and `LOCALAPPDATA` back.
- One case is not solved: your own C or C++ files (see [the Win32 page](windows-interop.md)) below a non-ANSI folder, on a volume where
  the creation of 8.3 names is switched off. The compiler then says which file it cannot open.
- **Case.** Windows compares file names without case: `os.Stat("aBC.txt")` finds `Abc.txt`, and two names that differ only in case are one file.
- **Links.** `os.Symlink` on Windows needs Administrator rights or Developer Mode (gocpp asks for the unprivileged-create flag like Go does).
- **Long paths.** The programs gocpp builds carry a `longPathAware` manifest (the plain C calls then take long paths where the system switch
  `LongPathsEnabled` allows it), and Go's `os` code prefixes `\\?\` itself, in `gocpp.exe` too (`gocpp.exe` has no manifest of its own), so a path
  of more than 260 characters works in them (a 317-character path was tested). A package's own C or C++ source below such a
  path may not open in a compiler that is not long-path aware (the generated C++ lives in the cache and is fine), and an
  argument that starts with `\\?\` is not interpreted by gocpp.
- **A `TEMP` of about 255 characters or more** is an operating system limit (252 works; 258 and 262 hang `gocpp.exe version` and `hostname.exe` on the
  test PC, until they are killed). gocpp cannot work around that, since the process
  never starts. Keep `TEMP` and `TMP` short.

Source files may use `\r\n` line ends (the scanner reads them like `\n`; a raw string drops the `\r` as in Go), and `gocpp fmt`
writes `\n` like gofmt. `gocpp.mod` may have either too.

## Console, UTF-8 and exit codes

- **Text.** A program writes UTF-8. To a pipe or a file the bytes go as they are, with `\n` staying `\n` (the C library's text mode
  is off), so output is the same as on Linux. To a console window the runtime uses `WriteConsoleW` after converting to UTF-16, so
  `héllo 世界` shows correctly in Windows Terminal and in the old console host with a suitable font. The console's code page is left alone, as
  in Go: C code that calls `printf` through a console shows the active code page (437 or 850 on many systems). The
  console path was run in a pseudo console over `ssh -tt`; a person typing at a real console window is on the checklist of the Windows
  test infrastructure.
- **The program's code page is UTF-8.** gocpp links an application manifest into every program (`activeCodePage` UTF-8,
  `longPathAware`, `asInvoker`, Windows 10). Without it a Windows program's ANSI code page is 1252, and a
  command-line argument, an environment variable or a file name with a non-ASCII character reaches C code damaged (`h?llo`); with it the
  narrow functions and `fopen` read UTF-8 as it is. The UTF-8 setting needs **Windows 10 version 1903 or later**; on an older system it is
  ignored. `GOCPP_NO_MANIFEST=1` leaves the manifest out. With GCC, gocpp's manifest replaces MinGW's default one (two manifests in one
  file would be an error); if gocpp cannot find a folder for that, it warns and the program gets MinGW's default.
- **Exit codes.** `gocpp run` returns the program's status, like Go's `go run`. Measured: `os.Exit(3)` gives 3;
  a panic prints Go's message and a `goroutine 1 [running]:` trace with `/` in the file names and gives 2; a compile error gives 1; a stack
  overflow prints `fatal error: stack overflow` and gives 2; an access violation, a division by zero or an illegal instruction in C code becomes
  Go's report (`panic: runtime error: invalid memory address or nil pointer dereference`, `[signal 0xc0000005 ...]`) and gives 2. Ctrl-C keeps
  the system's default, so a program ends with `0xC000013A` (`STATUS_CONTROL_C_EXIT`), as a Go program without `os/signal`.
- **Arguments** are split by the rules of Go's own Windows code (`CommandLineToArgvW` as Go uses it) and converted to UTF-8; the
  environment is read as UTF-16 and converted too.

## Environment variables

`gocpp env` prints every variable gocpp reads, with its value in use (`set NAME=value` lines on Windows, with `-json` as JSON). Set them
for one PowerShell session with `$env:NAME = 'value'` (`set NAME=value` in `cmd.exe`) or for good with *Edit environment variables for your account*.
The ones that matter on Windows:

| Variable | What it does |
|---|---|
| `GOCPP_CXX`, `CXX` | the C++ compiler, ahead of everything else (`GOCPP_CXX` first). Words are allowed: `GOCPP_CXX="ccache g++"` |
| `GOCPP_TOOLCHAIN` | `auto` (default), `local` or `pinned`: where the compiler comes from (above) |
| `GOCPP_TOOLCHAIN_URL` | a mirror for the download: an `https://` folder that holds the archives under their names |
| `GOCPP_OFFLINE` | `1`: never use the network (no toolchain download, no module download) |
| `GOCPP_NO_MANIFEST` | `1`: link no application manifest |
| `GOCPP_CFLAGS`, `GOCPP_CXXFLAGS`, `GOCPP_LDFLAGS` | your own flags for the compiler and the linker. A `-DUNICODE` or `-UNOMINMAX` here switches off that one [Win32 default](windows-interop.md#the-defaults-gocpp-sets) |
| `GOCPP_INTEROP` | `typed`, `opaque` or `auto` (the `-interop` flag wins): how C/C++ names are understood. The default is `auto`: typed when a clang++ is found (the pinned llvm-mingw has one), else `opaque`, with no message; `opaque` turns typed mode off (see the end of this page). The older `GOCPP_TYPED_INTEROP=1` still works |
| `GOCPP_CLANG` | the `clang++` of typed interop |
| `GOCPP_OS`, `GOCPP_ARCH`, `GOCPP_EXEC` | the target of a [cross build](windows-cross.md) and the program that runs its result |
| `GOCPP_LSP_CACHE` | the language server's cache, by default `%LocalAppData%\gocpp\lsp` |
| `LOCALAPPDATA`, `TEMP`, `TMP` | where the cache and the work folders are (above) |

The full list is in [the build spec](../spec/build.md#gocpp-list-and-gocpp-env). `GOCPP_GATE_PHASE` is a switch for gocpp's own
development: parts of the standard library and of the Windows interop (`C.wstr`, `os.Getenv`) are behind it until their phases are finished, and
this page's [interop examples](windows-interop.md) say which `GOCPP_GATE_PHASE` they need.

## Virus scanners and files in use

Windows lets another program hold a file open for a moment: a virus scanner that looks at a file that was just written, the search
indexer, a still-running program. gocpp expects it:

- every rename into the cache (the runtime library, objects, the toolchain download) is repeated with a growing wait for about three seconds
  when Windows answers *Access is denied* or a sharing violation; the download's unpacked folder is moved into place the same way;
- two `gocpp build` at once on the same cache share the work (`LockFileEx` locks, which the system releases when a process dies);
- each compiler runs in a job object that ends its whole process tree when gocpp is canceled or killed, so a stopped build leaves no `cc1plus.exe` behind;
- you cannot replace an `.exe` while it runs. `gocpp build` retries, then fails with `Access is denied. (is <path>\sleep.exe running? Windows cannot
  replace a program that runs: stop it, or write the build to another name)` (checked on the PC);
- the language server reads a file again in the background when another program holds it locked for more than half a second.

These are read from the code and its tests (they run on the PC, but no run involved a real scanner). If builds are slow or fail now and then with
*Access is denied* and a scanner is the suspect, exclude `%LocalAppData%\gocpp` from its real-time scan.

## Programs and libraries

- `gocpp build` makes a console program. For a program with windows and no console, pass the linker flag yourself: `gocpp build -ldflags=-mwindows`
  gave an executable with the GUI subsystem on the PC (the PE header says 2; without the flag it says 3, console). The flag is a trusted one (like
  `GOCPP_LDFLAGS`) and gocpp does not do anything else for such a program: a GUI program has no stdout for `fmt.Println`.
- `gocpp build -buildmode=c-shared -o mylib dll.gp` makes `mylib.dll` (exporting the `//export` functions and nothing else),
  `mylib.dll.a` and `mylib.lib` (import libraries for GCC and for MSVC's linker) and `mylib.h`. `-buildmode=c-archive` makes `mylib.a` and `mylib.h`.
  Both were built on the PC (the repository's test also links a C program against the DLL; see the [libraries guide](libraries.md) for calling gocpp from C).

## Editing on Windows

`gocpp.exe lsp` is the language server, and VS Code runs it as on Linux: install the extension from the repository (**Developer: Install Extension from Location...**
in the Command Palette, and pick the `editors/vscode` folder; [its README](../../editors/vscode/README.md) has the settings) and open a folder with `.gp` files.

- If `gocpp.exe` is not in the `PATH` of VS Code, set `gocpp.path`. On Windows `.exe` may be left out, the value can be the folder
  that holds `gocpp.exe` or a UNC name, spaces need no quotes, and `/` and `\` both work. A `.bat` or `.cmd` wrapper does not work (VS Code starts the binary without a shell).
- The server needs the same MinGW-w64 compiler as `gocpp build` for files that import a C or C++ header. Without one, each header import gets the error
  "cannot check the header imports" and the rest works. `gocpp env GOCPPCXX` shows what it finds.
- Names of files: `file:///C:/...`, VS Code's `file:///c%3A/...`, UNC names, folders with spaces and non-ASCII letters, 8.3 short names, `\r\n` line ends
  and a byte order mark all work, and the server gives every file one spelling, the one the disk has. What is not handled (links and junctions,
  `\\localhost\c$`, trailing dots, case-sensitive folders) is listed in [the editors guide](editors.md#windows).
- Speed: every budget of the language server is met on the PC (16 threads); the numbers are in
  [the Windows benchmark](../benchmarks/lsp-windows.md). The one miss is memory: the 91,000-line `large` project used 442 MiB against a budget of 400 MiB for 50,000 lines, and Linux shows the same.

**Debugging** works with gdb (step 14.6): `gocpp debug` finds the gdb of an install (PATH, MSYS2, Chocolatey, Scoop,
WinGet's WinLibs, then gocpp's llvm-mingw) before it builds anything, builds with DWARF debug information, and starts it with the
program as gdb's safe path (forward slashes, `;` as the list separator), so the gocpp script inside the program (the
`.debug_gdb_scripts` section of the `.exe`) loads and the printers work. On the PC, with WinLibs' gdb 17.2, in a folder and a
`TEMP` with spaces, in batch mode, these were checked: a breakpoint on a `.gp` line, `bt` with Go names and `.gp` lines, the printers
for strings, slices, maps and `int`, `gp locals`, `gp goroutines` (seven goroutines of every state), a stack overflow that stops at its `.gp` line,
and `catch throw` that stops a Go panic at the function that caused it. `gocpp debug -vscode` prints the VS Code `launch.json` (cppdbg with
`miDebuggerPath` and the safe path; `-debugger lldb` prints one for lldb-dap). Not tried on Windows: lldb and lldb-dap (the PC has no lldb), a launch in
VS Code, and stepping through a long session. Run `gocpp debug` from a terminal that gdb can take over: without `-batch` and without a
terminal it waits for input. A bare `bt` on a stack-overflow crash walks a hundred thousand frames: use `bt 5`. See the
[debugging guide](debugging.md#debugging-on-windows).

## What does not work yet

Each line was checked on the PC unless it says otherwise.

The native `fmt.Scan*`, `fmt.Fscan*` and `fmt.Sscan*` families are now available on Windows
(ADR-100). Six focused scanner/lowering regressions passed on the real Windows PC after the
native-fmt audit; this is not a claim that the full Windows or upstream fmt suite has run.

| What | State |
|---|---|
| `os/exec`, `os.StartProcess`, `os/signal`, `net` | **not available yet on any platform**, not only on Windows (Phase 9): `package os/exec is not available yet (Phase 09)`, `os.StartProcess` returns `fork/exec x: function not implemented`. Reading standard input works: `bufio.Scanner(os.Stdin)` from a pipe was checked on the PC. Files, directories, environment, links, `os.Exit` and the other parts of `os` work |
| time zones | `time.Local` is UTC and `time.LoadLocation` has no Windows data (Go reads the registry; gocpp does not yet) |
| `-race` | not for Windows: `gocpp build: -race is not supported for windows`. ThreadSanitizer does not exist for MinGW |
| `-asan` | not for Windows: `gocpp build: -asan is not supported for windows`. The MinGW GCC of WinLibs has no libsanitizer (`cannot read spec file 'libsanitizer.spec'`). Sanitizers run on Linux. clang's MinGW target has AddressSanitizer; gocpp has not tried it |
| lldb, lldb-dap, a VS Code launch, a long stepping session | not tried on Windows: the PC has gdb only. The `launch.json` for them is printed by `gocpp debug -vscode` and checked as text only (see above) |
| typed interop (`GOCPP_INTEROP=typed`, `-interop=typed`), `gocpp doc` of a header package | the probes run a clang for the Windows target: `x86_64-w64-mingw32-clang++`, the clang++ of the build or of the pinned llvm-mingw, or the clang++ in PATH, with `--target`, `-D_WIN32_WINNT=0x0A00`, the Win32 defaults and, on a system other than Windows, the headers of MinGW-w64's g++ (see the end of this page). Checked on Linux with MinGW-w64 headers (`<windows.h>` and its types are probed as LLP64; a typed program that imports it runs on the PC), and on the PC with the pinned llvm-mingw's clang (`auto` is typed, the probe, a typed build and `gocpp doc <windows.h>`; `GOCPP_TEST_DOWNLOAD=1 go test -run TestWindowsTypedNative ./tests`). On a Windows host `auto`, which is the default, uses the clang++ it finds (PATH, the build's compiler, the pinned llvm-mingw once it is installed) and stays `opaque` when there is none (a program built only with MSYS2's or WinLibs' g++ is built as before); `typed` without a clang exits with status 3 on `build`, `run` and `check` (1 on `gocpp test` for now) |
| Win32 programs with a window | `examples/win32` ran on the PC's console desktop (2026-10-21) and printed `output.txt`; over `ssh` alone (session 0) a window cannot show: see [the Win32 page](windows-interop.md#a-window) |
| a function pointer from `GetProcAddress`, `NewCallback`, COM classes of your own, the registry beyond the raw `advapi32` calls | no gocpp form for the first two; see the limits on the Win32 page |
| Dear ImGui (Win32, Direct3D 11) | `demos/imgui-dx11`: the window, its procedure and the message loop in gocpp (`<windows.h>`), the D3D11 device and swap chain through `<d3d11.h>` (COM methods), ImGui's own Win32 and DX11 backends built by a `cxx` recipe; ran on the PC's console desktop (2026-10-21, seen by the user). One header of its own, because `imgui_impl_win32.h` keeps `ImGui_ImplWin32_WndProcHandler` inside `#if 0` |
| Dear ImGui (GLFW, OpenGL 3) | `demos/imgui` is built from the library's source by a `cxx` recipe (`gocpp get`), linked with MinGW-w64's GLFW, and ran on the PC's console desktop with `glfw3.dll` beside it (2026-10-21, seen by the user: the widgets and the click counter work) |
| SDL2, raylib | link; `examples/raylib` ran on the PC's console desktop with `libraylib.dll` and `glfw3.dll` beside it (2026-10-21, a window with a bouncing ball until it was closed); SDL2 ran too, with `SDL2.dll` and `SDL3.dll` beside it (the MinGW sdl2-compat build loads SDL3): the hidden-window test program `tests/testdata/windows/libsdl2.gp` (exit 0) and a scratch program with a visible window and a bouncing box, which was seen on the desktop |
| Visual Studio's ABI (`clang-cl`, `cl.exe`), PDB files, natvis | not supported; MinGW-w64 only. C libraries built by Visual Studio link when they have a MinGW-compatible import library; C++ ones do not |
| 32-bit Windows, arm64 Windows | no |
| a console window that a person types at | not covered by an automated run (a pseudo console is) |

## Where to go next

- [Building for Windows from Linux](windows-cross.md): `-os windows`, running the `.exe` on a PC or under Wine.
- [Calling the Win32 API](windows-interop.md): `<windows.h>`, wide strings, `MessageBoxW`, a window, LLP64.
- [Editors and the language server](editors.md#windows): the details of paths and URIs.
- [FAQ](../faq.md#does-it-work-on-windows)
