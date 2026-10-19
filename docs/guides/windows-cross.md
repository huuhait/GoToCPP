# Building for Windows from Linux

> `gocpp build -os windows` turns a program into a `.exe` that runs on Windows 10 or later with no
> DLLs next to it. How it works: [docs/spec/build.md](../spec/build.md#cross-compilation) and
> [ADR-055](../adr/055-cross-compilation-targets-and-toolchain.md). This is the build side (Phase 06
> step 6.13); the Windows runtime is partly built, see "What works" below. To run `gocpp.exe` on Windows itself,
> see ["gocpp.exe on a Windows host"](../spec/build.md#gocppexe-on-a-windows-host).

## Build

```
gocpp build -os windows hello.gp        # writes hello.exe
gocpp build -os windows -arch amd64 -o out/ .
GOCPP_OS=windows gocpp build .          # the same, from the environment
```

Only `windows/amd64` is a Windows target. The program is linked with `-static`, so it needs no
`libstdc++-6.dll` or `libgcc_s_seh-1.dll`, and its main thread has an 8 MiB stack, like on Linux.

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

On Linux, with Wine installed:

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

See the table in [docs/spec/build.md](../spec/build.md#what-works-on-windows):
output, arguments, the environment, exit codes, panics, goroutines, channels, `select`, stack overflow,
big programs, stack traces with frames. The rest of the OS layer (files, processes, signals) is
Phase 14's. The programs under `tests/testdata/windows` show each piece.

## Testing on Windows

```
GOCPP_WINDOWS_SSH=user@host GOCPP_TEST_TARGET=windows go test ./tests -run 'TestRunWindows|TestWindowsPrograms'
GOCPP_TEST_TARGET=windows go test ./tests -run 'TestRunWindows|TestWindowsPrograms|TestWineIsolation'   # Wine
```

With `GOCPP_WINDOWS_SSH` the programs run on a real Windows PC over ssh (key authentication; they land in
`%USERPROFILE%\gocpp-wintest\<run>\`, which is deleted at the end); without it, under Wine in an isolated
prefix (Wine is not Windows: Wine 11.0 on AVX-512 CPUs cannot dispatch a stack overflow, and that
program is skipped there). `TestRunWindows` builds the run tests listed in `tests/windows_sample.txt`
(42 of them) for Windows and runs them, with the same expectations as `TestRun`; `GOCPP_TEST_WINDOWS=all` runs every run test that
`tests/windows_skip.txt` does not name, and each skip says why. `GOCPP_WINE_JOBS` (default 4) limits the
programs that run under Wine at once, `GOCPP_REQUIRE_WINE=1` makes a missing wine or compiler a failure.
