# ADR-055: A build has a target; Windows programs are built with MinGW-w64 or a pinned, downloaded llvm-mingw

Status: accepted (2026-10-18), refines [ADR-021](021-platforms-linux-windows.md) and
[ADR-036](036-action-graph-and-up-to-date-check.md). (The number is 055: 045 and 053 to 054 were taken when this was written.)

## Context

Linux and Windows are gocpp's two operating systems (ADR-021). The Windows port is Phase 14, but its
tests, and `hello.exe`, have to exist long before that, so Phase 06 step 6.13 builds the build side:
which compiler, which flags, which file names, which cache keys, and where the compiler comes from on a
machine that has none (the Go tool needs nothing installed to build for Windows; gocpp needs a C++
toolchain, so it has to be able to fetch one).

## Decision

1. **The target** of a build is a GOOS/GOARCH pair from, in this order, `-os`/`-arch` (check, build, run
   and emit), `GOCPP_OS`/`GOCPP_ARCH`, `GOOS`/`GOARCH`, and the machine. `check` and `emit` accept every
   pair that `go tool dist list` knows (build constraints and file suffixes follow it); `build` and `run`
   accept `linux/amd64`, `linux/arm64` and `windows/amd64` and say so for any other pair.
2. **A compiler for another target** (`build.FindCXXFor`) carries its target (`CXX.GOOS`, `CXX.GOARCH`).
   The target is part of every key that names a compiler (the stamp, the object cache, the runtime's
   files: `writeCompiler`), and only then: the host's keys do not change. The target decides the flags
   (`-D_WIN32_WINNT=0x0A00`; `--target=<triple>` for a clang that does not name its target; for the link,
   `-static`, an 8 MiB main-thread stack, and `-pthread` for GCC) and the name of the program (`.exe`).
3. **Which compiler** for windows/amd64: `GOCPP_CXX` if set (it names the target's compiler); else the
   rule of `GOCPP_TOOLCHAIN` (package `internal/toolchain`): `auto` (default) a MinGW-w64 compiler in PATH
   (`x86_64-w64-mingw32-clang++`, then `-g++`), else the pinned llvm-mingw; `local` never downloads;
   `pinned` ignores PATH. linux/arm64 is best effort: `aarch64-linux-gnu-g++` from PATH or `GOCPP_CXX`.
4. **The pinned toolchain** is llvm-mingw (UCRT variant), release 20261006, one archive for each host
   that is compiled into the binary with its size and SHA-256. The download is over HTTPS only (or from
   `GOCPP_TOOLCHAIN_URL`, a folder with the archives, like `GOPROXY`; the tests use a TLS test server).
   A redirect may go to another host (GitHub serves release assets from a storage host; the hash vouches
   for the contents) but only over https, at most 10 of them; a stalled transfer (nothing received for
   60 seconds) stops. `GOCPP_OFFLINE` never downloads (a toolchain that is cached is used). The file is
   checked against the size and hash before anything is
   unpacked (a host without an entry, or with an empty hash, fails closed), unpacked by code that is
   part of gocpp (`internal/toolchain/xz`: an xz and LZMA2 decoder, read to the end of the stream so that the
   checks of the last block, the index and the footer run; `fetch.go`: a tar reader that refuses absolute names, `..`,
   hard links and symbolic links that leave the folder), through a temporary folder and one rename, under a lock file so that two
   builds download once, and made read-only. Only `gocpp build` and `gocpp run` download: `check`, `emit`
   and the language server use a toolchain that is already there, so an editor never starts an 84 MB
   download. On a system without the toolchain's dynamic loader (NixOS) gocpp says so before downloading.
5. **Running a program of another target** is `GOCPP_EXEC` (split into words like `GOCPP_CXX`; Go's
   `-exec`): `GOCPP_EXEC=wine gocpp run -os windows .`. Without it, gocpp says what to set.
6. **Tests.** `GOCPP_TEST_TARGET=windows` makes the harness build run tests for Windows and run them on a
   real PC over ssh when `GOCPP_WINDOWS_SSH=user@host` is set (`internal/winssh`, the harness's copy of
   14.1's runner: the main way; the variable is the only place that names the machine), else under Wine
   (`tests/windows_test.go`; Wine is not Windows: Wine 11.0 cannot dispatch a stack overflow at its own
   guard page on AVX-512 CPUs, so the runtime puts a guard page of its own higher up under Wine, and the overflow
   program passes there too), with Wine kept away from the user's home and desktop
   (`internal/wine`: its own prefix, HOME and XDG folders, `WINEDLLOVERRIDES`, no DISPLAY, a server that
   Close ends by its PID, then every process that still has the prefix in its environment) and proven so by
   a canary (`TestWineIsolation`). The default is a sample (`tests/windows_sample.txt`);
   `GOCPP_TEST_WINDOWS=all` runs every run test that `tests/windows_skip.txt` does not name.
7. **The Windows OS layer** is step 14.1's ([ADR-053](053-windows-os-layer-on-createthread.md)); this step
   only builds with it and tests the driver. The link asks for the 8 MiB main stack with
   `-Wl,--stack,8388608`; the runtime's `.drectve` asks for the same, and the two agree and survive
   `--gc-sections` (`TestWindowsDriverFlags`).

8. **The default of `GOCPP_TOOLCHAIN` is one constant**, `toolchain.DefaultMode`. It is `auto` today: a
   `gocpp build -os windows` on a machine with no MinGW-w64 downloads about 80 MiB, with one line on
   stderr and without asking. Setting it to `local` makes the download opt-in (`GOCPP_TOOLCHAIN=auto` or
   `pinned` allows it) and nothing else changes: the tests read the constant, the error text says how to
   opt in, and `gocpp env` shows the current default.
9. **Keys and the runtime library.** The target of a compiler is part of `CompilerID`/`writeCompiler`, the
   one function every key calls (the stamp, the object cache, the runtime's objects). Step 6.8's runtime
   library (`rt/<ver>/lib/<key>`) must take its key from the same function, so that the runtime of a
   Windows build never meets the Linux one; a key built another way would be a bug.

## Consequences

- `gocpp build -os windows` works on a machine with no MinGW installed (one download, cached), and with
  the dev shell's MinGW-w64 offline.
- A different llvm-mingw release is a change of two constants and two hashes; the hashes were taken from
  the release page's digests and, for the linux/amd64 archive, recomputed by downloading the file.
- The pinned toolchain's C runtime is UCRT, the dev shell's is msvcrt: the Wine tests cover msvcrt; the
  UCRT runs are Phase 14's, on real Windows.
- The decoder is 900 lines of Go that gocpp has to maintain; the alternative (shell out to `tar` and `xz`)
  would have made the verified download depend on the machine's tools.
- Two Wine layers exist, `internal/wine` (this step) and the helpers of `runtime/windows_test.go` (14.1), and
  two ssh runners, `internal/winssh` and `runtime/winremote_test.go`. Unifying them crosses the two jobs
  (a test file of package `rtfiles_test` can't be imported): a later cleanup could move 14.1's helpers
  onto these packages.
