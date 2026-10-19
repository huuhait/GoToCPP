# ADR-076: Package `os` on Windows is Go's own Windows code over a generic `Syscall`

Status: accepted (2026-10-19). Refines [ADR-067](067-os-on-posix-natives-and-go-errors.md) for Windows and carries out
[Phase 14 §2.3](../../plans/2026-10-13-gocpp/phase-14-windows/README.md).

## Context

Step 9.6 built `os` for Linux as copies of Go's files over small POSIX natives (ADR-067). Windows needs the same package, with the
same promise: the error values, their texts and the edge cases are Go's. On Windows those come from a large amount of Go code:
`syscall`'s `Errno` (the system's own numbers, `FormatMessage` for the texts), the wrappers of about 60 Win32 functions, `os`'s
long-path rules (`\\?\`), the `ERROR_*` to `ErrNotExist` mapping, reparse points for symbolic links, `FindFirstFile`-less directory
reading by `GetFileInformationByHandleEx`, the file index for `SameFile`, console UTF-8, Windows' sharing modes. Phase 14's README
sketched "a native `internal/syscall/windows` on `<windows.h>`" for this layer.

## Decision

1. **The bottom layer is Go's runtime interface, not a Win32 wrapper per call.** Go's `syscall` reaches the system through `Syscall`..`SyscallN`,
   `loadlibrary`, `loadsystemlibrary` and `getprocaddress`, which its runtime implements. Four natives implement them
   (`gp::syscall::win_syscall15`, `win_loadlibrary`, `win_loadsystemlibrary`, `win_getprocaddress`; `runtime/src/os/windows/win_syscall.cpp`):
   call a function pointer with up to 15 integer arguments (`win_syscall_n`: up to 42, as a slice) on the calling thread (a goroutine is an OS thread, ADR-007), clear `GetLastError`
   before and read it after, as `asmstdcall` does. Everything above is Go's code: `LazyDLL`/`LazyProc`, `mksyscall`'s output
   (`zsyscall_windows.gp`), `Errno`, `UTF16FromString` with WTF-8, `Open`, `Seek`, `Pipe` ...
2. **Files are Go's, cut to what is needed.** `syscall` (no sockets, DNS, certificates, registry, IOCP, `CreateProcess`, `NewCallback`),
   `internal/syscall/windows` and `sysdll` (the calls of `os` and `poll`), `internal/filepathlite` (`path_windows.go` whole), `os`'s
   `dir`, `executable`, `file`, `path`, `stat`, `sys`, `types` `_windows` files. Go's own selection by file name and `//go:build` line picks
   them; the Linux-only files that had no build line got `//go:build unix` (`syscall_unix.gp`, `env_unix.gp`, `dirent.gp`, `process_unix.gp`,
   `fd_unix.gp`, `fd_poll.gp`, `errno_unix.gp`). `stdlib/*/PORT.md` list each cut.
3. **`internal/poll` is the blocking reduction again** (ADR-067 §3): handles are opened without `FILE_FLAG_OVERLAPPED`, `ReadFile`/`WriteFile` wait in
   the kernel, `Pread`/`Pwrite` use an `OVERLAPPED` for the offset and put the file pointer back (Go's blocking-handle path), the console is
   read and written as UTF-16 by Go's code. Unlike Linux, `Close` of a pipe wakes the threads blocked on it (`CancelIoEx`), as Go's
   overlapped pipes do.
4. **`os.Process` is rewritten** (`process_windows.gp`): a handle that `FindProcess` opens, `Kill`, `Wait`, `Release` with Go's states and
   its `EINVAL` on a second `Release`. `StartProcess` is `ENOSYS` as on Linux (os/exec later). `os.Args` is the runtime's (already split by Go's
   rules, ADR-053).
5. **`runtime.GOOS`/`GOARCH` come from `internal/goos`/`internal/goarch`** (files chosen by the target), not constants: `os` and every other
   package that switches on `runtime.GOOS` (`UserHomeDir`, `ProcessState`) now follow the target.
6. **Names that collide with the C library's macros.** The MinGW-w64 headers that `gocpp/rt.hpp` pulls in define 756 more macros
   (`DUMMYUNIONNAME`, `CLK_TCK`, `_alloca` ...) that a Go name must not be. They are added to `internal/lower/cxx_macros.txt`
   (the list is for all targets: a name does not depend on the target, and a Linux program with a variable named `DUMMYUNIONNAME`
   gets an underscore too); `TestMacroList` now reads the Windows compiler's list when MinGW is installed. UCRT's headers (llvm-mingw)
   may define a few more; a Go name that hits one fails to compile with a clear error, and `-update` adds it.
7. **Testing.** Real Go is the reference: `tests/run/os_win_*.gp` (`//go:build windows`) hold the output of Go 1.25.14 built for
   Windows and run on the real PC; gocpp's output on the same PC must equal it. 43 `os_*` tests that already pass unchanged join the Windows sample;
   the others have a twin or a reason in `tests/windows_skip.txt`. Everything runs over ssh on the PC (`GOCPP_WINDOWS_SSH`).

## Alternatives

- **A native C++ wrapper per Win32 function** (the plan's sketch, and what Linux does with POSIX): about 60 functions with their struct
  layouts and error conventions repeated by hand, and a second source of truth for the `Errno` texts. Rejected: the generic call keeps
  Go's wrappers, which already handle each function's failure value (`[failretval==InvalidHandle]`) and its `ERROR_ALREADY_EXISTS` oddities.
- **Port `internal/poll` whole with IOCP** (a completion port serviced by a thread): weeks of work for deadlines on files that Go
  cannot poll either; the blocking reduction costs nothing in what `os` promises.
- **`windows.h` through the header-package mechanism** (Phase 04): the generated C++ would include the Windows headers in every program
  and every Go name that is a Win32 macro would need protection (`ERROR_FILE_NOT_FOUND`, `DELETE`); this way the runtime's headers stay free of them.

## Consequences

Good: the Windows `os` has Go's behavior by construction; 101 of 114 existing `os_*` tests give the output of real Go on Windows, and the
others are Linux-only API (`Umask`, `Dup2`, `Pipe2`) or file names that Linux allows; the same code path serves a later
`os/exec`, `os/signal`, `net` (their Go files call the same `Syscall` layer).

Bad: struct layouts of Go and Windows must agree and the compiler's `static_assert(sizeof ...)` is the only check (`WIN32_FIND_DATAW`,
`FILE_ID_BOTH_DIR_INFO`, the reparse buffers are exercised by the tests); a call with a bad argument crashes instead of failing; `SyscallN`
takes 42 arguments (Go's limit; 15 without a slice) and cannot return floating point values; `NewCallback` does not exist (no window procedures from Go yet); the first
use of an exported function loads its DLL (a lock per call, cheap beside the system call); a program without the long-path
manifest cannot `Chdir` into a path of more than 260 characters (the manifest is a build-driver job: step 14.2); `go vet`-like pointer
rules are not checked.
