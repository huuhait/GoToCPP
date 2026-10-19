# ADR-081: The build driver on a Windows host: MinGW-w64 detection, job objects, an application manifest, DLLs

Status: accepted (2026-10-20). Carries out [Phase 14 §2.2](../../plans/2026-10-13-gocpp/phase-14-windows/README.md) and builds on
[ADR-055](055-cross-compilation-targets-and-toolchain.md) (the cross build and the pinned llvm-mingw),
[ADR-053](053-windows-os-layer-on-createthread.md) (the runtime's Windows layer) and [ADR-076](076-os-on-windows-over-a-generic-syscall.md)
(package `os`).

## Context

ADR-055 made `gocpp build -os windows` work from Linux. The next step is `gocpp.exe` itself on Windows: a user installs one file, runs
`gocpp build`, and gets an `.exe`. The 14.3 job had already run a cross-built `gocpp.exe` on a real PC with `GOCPP_TOOLCHAIN=local` and
`GOCPP_CXXFLAGS=-D_WIN32_WINNT=0x0A00`, and listed what was missing. Most of the driver was written for Linux: the compiler is found by
the names `clang++` and `g++`, a process tree is killed through a process group, the locks are `flock`, `os.Rename` is assumed to work, the
library modes are `.so` only, and the paths are compared as strings.

## Decisions

1. **A Windows host is a Windows target.** `FindCXXFor(windows, amd64)` no longer short-cuts to the host rule `clang++`, `g++`: on a Windows
   host it is `GOCPP_CXX`, `CXX`, then the rule of `GOCPP_TOOLCHAIN` (ADR-055), and the compiler it returns carries `GOOS=windows`, so the
   target's flags apply on a Windows host exactly as in a cross build: `-D_WIN32_WINNT=0x0A00`, `-static`, the 8 MiB stack, `-pthread` for
   GCC. A plain `clang++` gets `--target=x86_64-w64-mingw32`.
2. **Detection finds MinGW-w64 where Windows users have it** (`internal/toolchain/detect.go`). The order: the prefixed names in PATH
   (`x86_64-w64-mingw32-clang++`, `-g++`: llvm-mingw, distributions); the plain `clang++` and `g++` in PATH, **after asking the compiler what it
   builds** (`-dumpmachine` must be `x86_64-w64-mingw32` or `x86_64-w64-windows-gnu`: Visual Studio's LLVM `clang++` is `windows-msvc`, MSYS2's own
   `g++` is `x86_64-pc-msys`, Cygwin's `x86_64-pc-cygwin`, and none of them makes a MinGW program); then the folders where installs live when PATH
   does not name them (MSYS2 `ucrt64`, `clang64`, `mingw64` under `MSYS2_ROOT` or `C:\msys64`; Chocolatey's `mingw` and shims; Scoop's apps and
   shims; WinGet's WinLibs; `C:\mingw64`, `C:\w64devkit`, `C:\winlibs`); then the pinned llvm-mingw. PATH always wins over a folder: the user's PATH
   is their choice. `gocpp env` shows the compiler (`GOCPPCXX`) and where it came from (`GOCPPCXXSOURCE`). Every fact the search reads (environment,
   files, the compiler's answer) is a field of `toolchain.Config`, so the tests run it on fake layouts, on Linux.
3. **The pinned download for a Windows host is a zip** (`llvm-mingw-20261006-ucrt-x86_64.zip`, 190,836,485 bytes, SHA-256 in the binary, checked
   before anything is unpacked; digest from GitHub and from the downloaded file). `unpackZip` has the rules of the tar unpacker (no absolute or `..`
   path, no backslash, no link or device, one top-level folder, 8 GiB limit, writes through `os.Root`). The folder is renamed into place with
   `fsretry.Rename`; on Windows the folders are not made read-only (the attribute means nothing for a folder there, and a read-only folder cannot
   be removed by `RemoveDirectory`), the files are.
4. **File locks are real on Windows** (`internal/filelock`): `LockFileEx` (exclusive, fail immediately) on one byte far into an empty marker
   file, with the same semantics as `flock` where the callers need them (whoever holds it is the one build; the system releases it when the process
   dies). The four places that had a no-op (`internal/toolchain`, `internal/build` runtime library, `cmd/gocpp` generated folder, `internal/modfetch`)
   use it. Two `gocpp build` at once on the same cache now do the work once.
5. **`fsretry.Rename` for every rename into a cache.** `MoveFileEx` fails with *Access is denied* or a sharing violation while a virus scanner,
   the search indexer or a still-running program holds the file. The rename is repeated with a growing wait for about three seconds, only for those
   errors; a folder onto an existing folder is not repeated (it never works; the callers read the failure as "another process was first"). On other
   systems it is `os.Rename`.
6. **Job objects are the process group** (`internal/build/proc_windows.go`). `InGroup` asks for one, `Start` puts the started process into a job
   created with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` (`joinGroup`), canceling the command and `KillGroup` terminate it. Because the limit is
   *kill on close*, the system also ends the compiler's tree when gocpp itself is killed or crashes: the handle goes with the process. A job nests
   (Windows 8+), so a gocpp that is in a job of a CI service still works. One known window: a child that the compiler starts in the first
   milliseconds, before `AssignProcessToJobObject` runs, is outside the job (os/exec does not give the thread handle, so a suspended start is not
   possible); compiler drivers spend that time loading.
7. **Command lines go through response files when they are long** (`internal/build/rsp.go`, `fitCommand`): any command whose line (program,
   its own arguments and the rest) would pass 30,000 characters (CreateProcess takes 32,767) puts all its arguments into `cmd-N.rsp` in the work folder.
   The link had its own rule since 6.9 (the driver flags stay on the line). A Windows target writes the path separators in a response file as
   `/`: GCC reads a backslash as an escape, LLVM's Windows tokenizer does not, and both read `/`. (The cost: a literal backslash inside a flag such as
   `-DX="a\b"` is read as `/`; no real flag is affected, and the test says so.) `os/exec` quotes the command line for CreateProcess itself; nothing goes
   through a shell.
8. **The application manifest** (`internal/build/manifest.go`). A Windows program gets a manifest resource with `longPathAware`, `activeCodePage` UTF-8,
   `requestedExecutionLevel asInvoker` and `supportedOS` Windows 10, linked as one small COFF object that gocpp writes itself (two sections `.rsrc$01` and
   `.rsrc$02` with one `ADDR32NB` relocation, as windres writes them), so no resource compiler is needed and GNU ld and lld take it alike. **Go's own
   linker writes no manifest** (checked in Go 1.25's `cmd/link`; the 14.3 note that it does was wrong, and is corrected in the Phase 14 README).
   **Default on, with `GOCPP_NO_MANIFEST=1` as the way out**, because without it a gocpp program loses data on a normal Western system: the ANSI code
   page is 1252, and the C and C++ side reads `char` strings in it. Measured on the real PC: without the manifest `GetACP()` is 1252, a
   command-line argument or environment variable `héllo` reaches C code as `h?llo`, and `fopen(go_string)` with a non-ASCII name opens another
   file or none (the string is UTF-8, the file system call reads it as 1252); with the manifest `GetACP()` is 65001 and all of that is exact.
   The UTF-8 code page needs **Windows 10 1903 or later** (an older system ignores the setting, and the program behaves as without it).
   The other settings: `longPathAware` lets the plain Win32 and C library calls take paths over 260 characters (with Windows' `LongPathsEnabled`;
   Go's `os` prefixes `\\?\` itself, so it never needed it); `asInvoker` stops Windows' guess that a program called `setup.exe` or `update.exe`
   needs administrator rights.
   *Linking it.* clang (llvm-mingw, MSYS2's clang64) takes the object as an input file. **GCC links MinGW's own `default-manifest.o` into every
   program** (its specs: `%{!shared:%:if-exists(default-manifest.o%s)}`); two manifests in one program are an error that GNU ld does not report (it
   keeps the first and leaves the other as garbage: "Extra data in .rsrc section"). So with GCC gocpp's object is a file of that name in a folder that `-B`
   puts before GCC's, and the driver links ours *in the place of* its default. **The `-B` folder must be absolute, ASCII and without a space**: a `-B`
   prefix is expanded into spec text that is split at spaces and read in the ANSI code page, and a relative folder is not looked at by the spec function at all
   (measured with GCC 16.1: the program then had neither manifest). The first version, an absolute folder in the cache, broke every GCC link when
   the cache path had a space (`C:\Users\John Smith\AppData\Local\gocpp`: "cannot find C:/Users/John"; found by the review, and the tests had
   relocated the cache into a folder without spaces). So `manifestLink` takes the first of these that is plain and can be written: the cache folder, its
   8.3 short name, the work folder (and its short name), the private folder `%ProgramData%\gocpp-paths-<SID>\manifest-<hash>` (decision 13);
   and when there is none, no `-B` flag at all and a visible warning that the program is built without the manifest (MinGW's default manifest is used).
   A first version of the object had a wrong symbol number in the relocation: GNU ld accepted it, lld produced an `.exe` that Windows refused to start;
   `TestManifestObject` reads the object with `debug/pe` now.
9. **DLLs.** `-buildmode=c-shared` for windows/amd64 writes `x.dll`, the import library `x.dll.a` (what `-lx` finds; the linker writes it with
   `--out-implib`) and a copy `x.lib` (MSVC's link.exe and lld-link), and the header `x.h`. Without `-o` the DLL is `<program>.dll` (no `lib`
   prefix: it is named like a program); `-o` names it, and an `-o` without an extension gets `.dll` (`.a` for `c-archive`), as `-o prog` gets `.exe`:
   `-o mylib` writes `mylib.dll`, `mylib.dll.a`, `mylib.lib`, `mylib.h`. A DLL asked for as `.so` (or any extension but `.dll`) is an error that says so.
   The header's link line is `-l<name>` with the name as it is (a Linux library `libx` is `-lx`; a DLL `lib.dll` is `-llib`). The exports are the `//export` functions, marked `__declspec(dllexport)` by the glue and
   the header (`GOCPP_EXPORT`, there since 6.10): the linker then exports nothing else, and a test counts the export table. No version script, no
   soname, no `-fPIC` (GCC says "ignored for target"), no `-fno-gnu-unique` on Windows. `c-archive` and `cpp-lib` are the same `.a` files as on Linux;
   the header's link line has no `-ldl`.
10. **`-o prog` names `prog.exe`** for a Windows target; a name with an extension keeps it. This differs from `go build -o prog`, which writes the name as given (cmd/go adds `.exe` only to the default name); gocpp does it because CreateProcess, cmd.exe and PATHEXT need the extension, and a DLL gets `.dll` for the same reason (9). `compile_commands.json` uses the `arguments` array, so
    Windows paths need no shell quoting.
11. **Paths are spelled one way** (`cmd/gocpp/pathcase_windows.go`). The same folder reaches gocpp as `c:\proj` (the shell), `C:\Proj\x` (a pattern)
    and the loader's walk upward, and the loader compares them as strings. gocpp's own entry points (`getwd`, absolute patterns) now canonicalize with
    `GetLongPathNameW` (the case of each existing element as stored, 8.3 names expanded) and a capital drive letter. Other systems: unchanged.
12. **Cache location.** `os.UserCacheDir` is `%LocalAppData%`, so the cache is `%LocalAppData%\gocpp` (`gocpp env GOCPPCACHE`) and the temporary
    files are `%TEMP%`; the tests relocate both by setting those variables. There is **no** `GOCPPCACHE` input variable: `GOCPPCACHE` stays a
    value that `gocpp env` finds out (the plan's text named it as the place, not as a switch); a switch would need every package that finds the
    cache folder to agree, and the six that do already follow `os.UserCacheDir`.

13. **Paths the compilers can read** (`internal/winpath`, `internal/build/safeargs.go`, `cmd/gocpp/envpaths.go`; found by the review, which put
    spaces and non-ANSI characters in the cache and in the user's folders). MinGW-w64's programs read their command line in the ANSI code page (1252 on the
    PC): a character outside it (a user called `日本`, a folder with an emoji) arrives as `?`, and nothing is found. A space is no trouble for an
    argument (each is one argv word); it is for text that GCC expands itself (the `-B` case above). Go's own file functions use the wide API and have no such limit,
    so gocpp can keep a path out of the compilers' arguments. The rule is "a path that a compiler gets must be ACP-safe", made so:
    - *The folders gocpp makes files in.* At the start of the process `LOCALAPPDATA` (the cache), `TEMP` and `TMP` (the work folders) whose value is not
      ACP-safe (or, for TEMP and TMP, is longer than 200 characters: CreateProcess takes a working folder of at most 247, and a work folder is
      started in) are replaced by the 8.3 short name of the same folder (`normalizeEnvPaths`). `gocpp env GOCPPCACHE` shows the one in use. A safe
      value, spaces included, is not touched. A name that is a legal 8.3 name already (`tmp_Ł`, a user `Łukasz`) has no short name, so these users take the next choice:
    - *The private folder below ProgramData* (`winpath.PrivateFolder`): `%ProgramData%\gocpp-paths-<SID>\<hash of the original value>\{LOCALAPPDATA,TEMP}`.
      `%ProgramData%` is shared by all users and what is made there inherits `Users: read` and `Users: create` entries (measured with icacls), so a plain
      folder would let other users read the temporary files and the cache, and a hostile user could make the folder first (the name is a hash of the
      victim's own path), own it, and put objects and headers into the victim's cache: code execution. So the folder (1) has the user's SID in its name, (2) is
      created with a protected DACL (no inheritance) that gives full control to the user, SYSTEM and Administrators and no one else, and (3) when it exists is
      used only if it is a plain folder (no junction or link), is owned by the user, SYSTEM or Administrators, and its DACL has no access entry (read or write)
      for anyone else; every level below the first is made and checked the same way. A folder that fails the check is refused (the variable stays as it is, or
      the manifest warns: see 8), never used. The manifest object of GCC's `-B` has its own such folder (`gocpp-paths-<SID>\manifest-<hash>`), one for each user.
    - *The user's program sees the user's environment.* `normalizeEnvPaths` keeps the original values, and a program that `gocpp run` or `gocpp test`
      starts gets them back (`programEnv`): `os.Getenv("TEMP")` is the same whether the program was started by gocpp or by hand (it printed `...\TMP_~1` under `gocpp run`
      before). The compilers, the linker and pkg-config get the replacements. The debugger of `gocpp debug` is a tool of the toolchain and gets them too (step 14.6).
    - *The user's own files and folders* (a package's `.c` and `.cpp`, headers, a `cxx` recipe below a folder `日本`): for a compiler that reads
      its command line in the ANSI code page (GCC before 14; a compiler of unknown version), `build.CompilerArg` replaces the path in an argument
      (`-I<dir>`, `-iquote`, a file, `@file`, `-ffile-prefix-map=a=b`) by its short name at the one place every compiler runs (`CXX.Command`) and in
      the response files. GCC 14 and later and clang read the command line as Unicode (measured with WinLibs' GCC 16.1: a source and an `-I` folder named
      `日本` work as they are), so they get it unchanged, and the diagnostics keep the real names; the unit tests run the mapping on a fake Windows, since
      the PC has no old GCC. What *every* GCC reads as bytes in the ANSI code page is the text of an `#include "<path>"` line: those that gocpp writes
      (`winpath.IncludeFile`, in `cppir`, the namespace probe, the C header check and the typed-interop probe) get the short form (measured: a `cxx` recipe
      below `日本 lib` failed with "clib.h: No such file or directory" before). And `as` and `ld` read and write files by the ANSI name: hence the cache and
      the work folder above.
    - *The names gocpp chooses*: the program (`prog.exe`) and a library are linked in the work folder under an ASCII name when the real one is not safe
      (`n_jp-日本.exe`: "ld: cannot open output file ... Invalid argument" before), and moved to the real name by Go's wide API; the objects of the
      sources `日本.c` are `gpsrc<N>.o`.
    - *No short name* (8.3 creation is off on the volume) *for a user's own file*: the path stays as it is, and the compiler says what it cannot open.
      This is the one case that is not solved; the cache and the work folders have the ProgramData fallback. Not covered: typed-interop probes and
      `gocpp doc` on a Windows host call the host's `clang++`/`g++` directly (`typedinterop/tsupport`, `probe/run.go`, `icheck`), not the Windows rule
      of this ADR (TODO for 14.6).
14. **A command line over the limit is measured as CreateProcess gets it**: the program and each argument quoted by the rules of `os/exec`
    (`escapedLen`, checked against a copy of `syscall.EscapeArg`), not their bytes; many short arguments with spaces pass 32,767 once quoted. The limit
    is 30,000 (tested at 29,999, 30,000 and 30,001).
15. **Replacing an output.** A read-only existing `prog.exe` is made writable first (`cmd/go` removes the old file; Windows refuses a rename onto a
    read-only file); a running program cannot be replaced, and the error says so ("is prog.exe running?") after the retries.
16. **A killed download leaves nothing for a day**: the download folder is `.download-<pid>-*`; the next download (it holds the lock of the
    toolchain folder) removes the folders of processes that are gone at once, keeps those of live ones, and sweeps old unnamed ones after a day;
    `gocpp clean -toolchain` removes the lock file, the leavings and the empty folder.

## Alternatives

- **Assign gocpp itself to one job object at start** (no window for early children, and everything gocpp starts dies with it, also the program of
  `gocpp run`). Rejected: the user's program should outlive a crash of gocpp, as on Linux, and a job that kills on close would also kill a daemon
  that a `gocpp run` program starts on purpose.
- **windres/llvm-windres for the manifest.** Needs a preprocessor and a resource compiler found next to the compiler, and differs between WinLibs,
  MSYS2 and llvm-mingw. A 300-byte object that gocpp writes needs none.
- **`-Wl,--exclude-all-symbols` / export lists** for DLLs: `dllexport` is the portable form and it is what the glue wrote already.
- **A GOCPPCACHE variable that moves the cache** (see 12).

## Consequences

Good: `gocpp.exe` on a machine with WinLibs, MSYS2, Chocolatey or Scoop builds with no setting, and on a machine with nothing it downloads one
file and builds; UCRT (llvm-mingw) is tested for real: the pinned toolchain's hello builds and runs on the PC. Two builds at once, killing gocpp,
a locked file and a long command line no longer break a build. Costs: one more object in every Windows link (and a changed `.exe` hash against
a build without it), and the UTF-8 code page is a behavior that Go programs do not have (a C library that calls the narrow Win32 functions sees
UTF-8; `GOCPP_NO_MANIFEST` is the way back). Not covered: `\\?\`-prefixed arguments to gocpp (a long path without the prefix works), compilers
that cannot open a source whose path is over 260 characters (a package's own C++ files in such a module: the generated C++ is in the cache and is
fine), a real console window (14.7's checklist).
