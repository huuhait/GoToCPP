# Building for Windows from Linux

> Testing on Windows (the whole suite on a real PC): [windows-testing.md](windows-testing.md).

> `gocpp build -os windows` turns a program into a `.exe` that runs on Windows 10 or later with no
> DLLs next to it. How it works: [docs/spec/build.md](../spec/build.md#cross-compilation) and
> [ADR-055](../adr/055-cross-compilation-targets-and-toolchain.md). This page is for a Linux machine that builds
> for Windows. To run `gocpp.exe` on Windows itself, see [gocpp on Windows](windows.md); to call the Win32 API,
> see [the Win32 page](windows-interop.md).

## Build

```
gocpp build -os windows hello.gp        # writes hello.exe
gocpp build -os windows -arch amd64 -o out/ .
GOCPP_OS=windows gocpp build .          # the same, from the environment
```

Only `windows/amd64` is a Windows target. The program is linked with `-static`, so it needs no
`libstdc++-6.dll` or `libgcc_s_seh-1.dll`, and its main thread has an 8 MiB stack, like on Linux. Without `-o`
the file is named after the program (`hello.exe`); `-o prog` writes `prog.exe` (a name with an extension keeps it;
`-buildmode=c-shared -o mylib` writes `mylib.dll` with its import libraries `mylib.dll.a` and `mylib.lib`, and `mylib.h`).
A file named `x_windows.gp`, and one with `//go:build windows`, is part of such a build and not of a Linux one.

`gocpp check -os windows` and `gocpp emit -os windows` stop before the C++ compiler, so a Windows-only package
(`examples/win32` for one) can be checked on Linux in a moment; a package that imports `<windows.h>` still needs the MinGW headers of
the cross compiler for the header check.

## Where the compiler comes from

1. `GOCPP_CXX`, if you set it (the compiler for the target; for example `x86_64-w64-mingw32-g++`).
2. A MinGW-w64 compiler in `PATH`: `x86_64-w64-mingw32-clang++`, then `x86_64-w64-mingw32-g++`
   (Debian and Ubuntu: `g++-mingw-w64-x86-64`; Fedora: `mingw64-gcc-c++`; MSYS2: the `mingw-w64-ucrt-x86_64-gcc`
   package; the nix dev shell has one).
3. gocpp's pinned **llvm-mingw** (UCRT), which `gocpp build` and `gocpp run` download on first use
   (about 84 MB, into `~/.cache/gocpp/toolchain/`, one line on stderr, no question), checked against a
   SHA-256 and a size that are part of the gocpp binary, over https only (a redirect to http is
   refused). `gocpp check` and the editor never download, and `GOCPP_OFFLINE=1` stops the download too
   (a toolchain that is already cached is used).

`GOCPP_TOOLCHAIN` changes the rule: `auto` (the list above, today's default), `local` (never download:
for offline machines and CI; the error says how to install a compiler) and `pinned` (always the
download, so that every machine builds with the same compiler). The default is the constant
`toolchain.DefaultMode`: if downloading without asking is not wanted, it becomes `local`, and
`GOCPP_TOOLCHAIN=auto` is then the opt-in. `gocpp env` shows the current default. `GOCPP_TOOLCHAIN_URL` points the
download at a mirror: an `https://` folder that holds the archives under their names. `gocpp clean
-toolchain` removes the download.

On NixOS the downloaded llvm-mingw does not start (it needs `/lib64/ld-linux-x86-64.so.2`): gocpp says
so before it downloads; use the dev shell's MinGW-w64 (`GOCPP_TOOLCHAIN=local`) there.

## Run

The usual way is to copy the `.exe` to a Windows machine and start it there. It needs nothing next to it: this `hello.exe`, built on Linux
with the dev shell's MinGW-w64, ran on a real Windows 11 PC as it was, and so did the cross-built `winapi.exe` (kernel32, advapi32 and user32 calls),
`wstr.exe` (wide strings) and a Winsock program of [the Win32 page](windows-interop.md). A cross build uses the C runtime of its compiler (msvcrt in the dev shell, UCRT in
the downloaded llvm-mingw); the `.exe` runs on every Windows 10 or later.

On Linux, Wine is optional. With Wine installed:

```
GOCPP_EXEC=wine gocpp run -os windows .
```

`GOCPP_EXEC` is the program that starts the `.exe` (it takes words: `GOCPP_EXEC="wine64 --"`),
like `go run -exec`. Without it, `gocpp run -os windows` says what to set instead of building first.

Wine writes to your home folder (`~/.wine`, desktop entries) and can open windows on your desktop.
gocpp's own tests keep it away with an isolated prefix (below); for your own runs:

```
export WINEPREFIX=$HOME/.cache/gocpp-wine WINEDEBUG=-all DISPLAY= WAYLAND_DISPLAY=
export WINEDLLOVERRIDES='winemenubuilder.exe=d;mscoree,mshtml=;winewayland.drv=d'
```

## linux/arm64 (best effort)

`gocpp build -arch arm64` uses `aarch64-linux-gnu-g++` from `PATH` (Debian: `g++-aarch64-linux-gnu`),
or the compiler in `GOCPP_CXX`; with a plain clang it passes `--target=aarch64-linux-gnu`. Run it with
`GOCPP_EXEC=qemu-aarch64`. The driver is tested with fake compilers; no CI job builds for arm64 until
Phase 15.

## What works

A cross-built program is the same program that `gocpp.exe` builds on Windows: output (UTF-8; a console gets `WriteConsoleW`), arguments, the
environment, exit codes, panics with Go's message and trace, goroutines, channels, `select`, stack overflow, big programs, and package `os`
(files, directories, links, environment, `Stat`; Go's own Windows code). The table is in
[docs/spec/build.md](../spec/build.md#what-works-on-windows) and the programs under `tests/testdata/windows` show each piece.

What a cross build cannot do is what Windows cannot do yet: time zones, `-race`, `-asan` (both are refused for a Windows target), and `gocpp test`
(the link of the `testing` package fails for Windows, from Linux too: `undefined reference to gp::testing::cpu_name()`). The list, with how each was
checked, is [on the Windows page](windows.md#what-does-not-work-yet). (`os/exec`, `os/signal`, `net` and `fmt.Scan*` are not available yet on any platform.)

## Libraries

C and C++ libraries for a Windows target must be built for MinGW-w64: a Linux `libz.a` does not link. `//gocpp:pkg-config` asks pkg-config with the target's
`PKG_CONFIG_*` variables of the environment. In the nix dev shell, `tools/winlibs.sh DIR` builds sqlite3, zlib, SDL2 and raylib for MinGW-w64 and the
tests find them with `GOCPP_WINDOWS_LIBS=DIR`. Libraries that Visual Studio built (a C++ ABI) are not supported.

## Testing on Windows

```
GOCPP_WINDOWS_SSH=user@host GOCPP_TEST_TARGET=windows go test ./tests -run 'TestRunWindows|TestWindowsPrograms'
GOCPP_TEST_TARGET=windows go test ./tests -run 'TestRunWindows|TestWindowsPrograms|TestWineIsolation'   # Wine
```

With `GOCPP_WINDOWS_SSH` the programs run on a real Windows PC over ssh (key authentication; they land in
`%USERPROFILE%\gocpp-wintest\<run>\`, which is deleted at the end); without it, under Wine in an isolated
prefix (Wine is not Windows: Wine 11.0 on AVX-512 CPUs cannot dispatch a stack overflow, and that
program is skipped there). `TestRunWindows` builds the run tests listed in `tests/windows_sample.txt`
for Windows and runs them, with the same expectations as `TestRun`; `GOCPP_TEST_WINDOWS=all` runs every run test that
`tests/windows_skip.txt` does not name, and each skip says why. `GOCPP_WINE_JOBS` (default 4) limits the
programs that run under Wine at once, `GOCPP_REQUIRE_WINE=1` makes a missing wine or compiler a failure.
