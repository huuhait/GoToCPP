# syscall

Step 9.6 (Phase 09). A **subset** of go1.25.14's `src/syscall` for Linux (BSD-3-Clause, see `../LICENSE`); `tools/portstd` does
not port it (assembly stubs, `//go:linkname`, the runtime's thread handling).

## Files

- `zerrors_linux_amd64.gp`: Go's file as it is: every constant, `errors` (the table of `Errno` texts: `no such file or
  directory`, lowercase, Go's own strings) and `signals`. The numbers are Linux's for amd64 and arm64 alike for the errors and
  signals; a few `O_*`/`F_*` flags differ on arm64 (`O_DIRECTORY`, `O_NOFOLLOW`, `O_DIRECT`, `O_LARGEFILE`): Phase 14/15 add
  `zerrors_linux_arm64.gp` with the `GOARCH` suffix.
- `syscall.gp`: `Errno` (`Error`, `Is`, `Temporary`, `Timeout`: Go's), `Signal`, `Conn`, `RawConn`, `SysProcAttr` and
  `Credential` (types only), `ByteSliceFromString`, `StringByteSlice`, `Stdin/Stdout/Stderr`, `ImplementsGetwd`.
- `ztypes_linux_amd64.gp`: `Timespec`, `Timeval`, `Rusage`, `Rlimit`, `Stat_t`, `Dirent`, `Utsname`, `Fsid`, `Statfs_t`
  (Go's definitions, field for field), `PathMax`, and the `Nano`/`Unix` methods and conversions of `timestruct.go`.
- `env_unix.gp`: Go's `env_unix.go` (the lock, the copy of the environment, `Getenv`, `Setenv`, `Unsetenv`, `Clearenv`,
  `Environ`): the environment has **one lock** (`sync.RWMutex`) and `Setenv` also calls the C library's `setenv`
  (`gp::syscall::runtime_setenv`) so that C and C++ code the program calls sees the change.
- `dirent.gp`: Go's `dirent.go` (`ParseDirent`, `readInt`) without the big-endian code.
- `syscall_linux.gp`: the system calls that `os` and its tests need, each a native function of the runtime
  (`runtime/src/os/linux/posix_file.cpp`, declared in `gocpp/syscall.hpp`) that returns the errno of a failure and **repeats a
  call that a signal interrupted (EINTR)** before Go code sees it. The exported functions turn the errno into an `Errno` (the
  boxed values `EAGAIN`, `EINVAL`, `ENOENT` are shared, as in Go). **Every `Open` adds `O_CLOEXEC`.** A path with a NUL byte is
  `EINVAL` before any system call (Go's `BytePtrFromString`). A call that returns a count returns -1 with the error of the
  system call, as Go's raw system calls do (`Read`, `Getcwd`, ...), and 0 with `EINVAL` for a path with a NUL byte (`Open`,
  `Readlink`: Go's `BytePtrFromString` fails before the call); `Uname` fills every field of `Utsname`.
- `wait_linux.gp`: `WaitStatus` (Go's) and `Wait4`.
- `Getrlimit` and `Setrlimit` (`syscall_linux.gp`) are the system's. `RLIMIT_STACK` shows the soft limit that the runtime
  raised when the program started (1 GiB, at most the hard limit; a larger one stays; D7), where Go shows the 8 MiB of the
  shell: like Go's own raise of `RLIMIT_NOFILE`, which `Getrlimit` shows too, it is not hidden. A `Setrlimit` of
  `RLIMIT_STACK` also moves the runtime's bounds of the main thread's stack (the stack guards of `encoding/json` and
  `path/filepath`, the overflow report). Programs that the process starts inherit the raised limit.

## Not built

`Syscall`, `Syscall6`, `RawSyscall`, `ForkExec`, `StartProcess`, sockets, `ptrace`, `mmap`, `Select`, `Sysinfo`, `Statfs`, `Flock`,
the `Sockaddr` family, `Exec`, `Chroot`, `Mount`, and everything for other systems. `tools/stubgen`'s
`stubPackage.Partial` says that the package declares only some of Go's names; every name it declares must have Go's type.

## Windows (step 14.3, ADR-076)

On Windows the package is Go's own, trimmed (go1.25.14, `src/syscall`), selected by the `_windows` file names: Go's files are copied as
they are except for the edits below, and the natives are only the bottom layer (`gocpp/syscall.hpp`, `runtime/src/os/windows/win_syscall.cpp`).

| File | Is | Differs from Go's |
|---|---|---|
| `dll_windows.gp` | `dll_windows.go` | `Syscall`, `Syscall6`, `Syscall9`, `Syscall12`, `Syscall15`, `Syscall18`, `SyscallN`, `loadlibrary`, `loadsystemlibrary` and `getprocaddress` (Go's runtime provides them) are Go functions over four natives: `win_syscall15` calls the function at an address with up to 15 integer arguments (`win_syscall_n`, with a slice, up to 42, Go's limit: `SyscallN` and `Syscall18` use it above 15) on the calling thread (a goroutine is an OS thread: no `cgocall`, no stack switch), clears `GetLastError` before the call and reads it right after, as Go's `asmstdcall` does; `win_loadlibrary`, `win_loadsystemlibrary` (`LOAD_LIBRARY_SEARCH_SYSTEM32`) and `win_getprocaddress`. `SyscallN` panics above 42 arguments (Go's limit); `r2` (Go: XMM0, for a function that returns a double) is always 0. `LazyDLL.Load` and `LazyProc.Find` always take their mutex (no atomic pointer loads). |
| `syscall_windows.gp` | `syscall_windows.go` | The `//sys` functions that are not taken (83 of 149): `createIoCompletionPort`, `getQueuedCompletionStatus`, `postQueuedCompletionStatus`, `CancelIo`, `CreateProcess`, `CreateProcessAsUser`, `getStartupInfo` (so `GetStartupInfo`), `CryptAcquireContext`, `CryptReleaseContext`, `CryptGenRandom`, `CommandLineToArgv`, `CreateFileMapping`, `MapViewOfFile`, `UnmapViewOfFile`, `FlushViewOfFile`, `VirtualLock`, `VirtualUnlock`, `TransmitFile`, `ReadDirectoryChanges`, the `Cert*` functions, `RegOpenKeyEx`, `RegCloseKey`, `RegQueryInfoKey`, `regEnumKeyEx`, `RegQueryValueEx`, the process attribute list functions, `SetFileCompletionNotificationModes`, all of Winsock (`WSA*`, `socket`, `bind`, `connect`, `Getsockopt`, `Closesocket` ...), DNS (`DnsQuery` ...), `GetAddrInfoW`, `GetIfEntry`, `GetAdaptersInfo`, `GetHostByName`, `GetServByName`, `GetProtoByName`, `Ntohs`, and the `security_windows.go` ones (`TranslateName`, `GetUserNameEx`, `NetUserGetInfo`, `NetGetJoinInformation`, `NetApiBufferFree`, `LookupAccountSid`, `LookupAccountName`, `ConvertSidToStringSid`, `ConvertStringSidToSid`, `GetLengthSid`, `CopySid`, `OpenProcessToken`, `GetTokenInformation`, `GetUserProfileDirectory`) with their Go wrappers (the sockets, `Sockaddr`, `Linger`, `LoadConnectEx` ..., `CreateIoCompletionPort`, `GetQueuedCompletionStatus`, `PostQueuedCompletionStatus`, the `procThreadAttributeListContainer`, `RegEnumKeyEx`, `GetStartupInfo`, `LoadCancelIoEx`, `LoadSetFileCompletionNotificationModes`); `NewCallback`/`NewCallbackCDecl` are gone too. `ReadFile`/`WriteFile` are Go's without the race/msan/asan hooks; `setFilePointerEx` only has the 64-bit branch (the Windows layer is x86-64: ADR-053); `Exit` is `gp::os::exit` (flush C's stdio, no deferred function); `Getpagesize` is 4096. |
| `zsyscall_windows.gp` | `zsyscall_windows.go` | Only the functions of the kept `//sys` lines (66, all kernel32); each is `mksyscall`'s output as it is. |
| `env_windows.gp` | `env_windows.go` | `runtimeSetenv`/`runtimeUnsetenv` are natives (`win_setenv`, `win_unsetenv`) that get the UTF-16 that `Setenv` made (so unpaired surrogates stay) and also put the variable into the C library's copy (`_wputenv_s`), so that C code the program calls sees `os.Setenv`; the C library deletes a variable that it is given empty, so the native sets the system's value again (C's `getenv` answers NULL for a variable that is set to the empty string). |
| `exec_windows.gp` | `exec_windows.go` | `EscapeArg`, `ProcAttr`, `SysProcAttr`, `FullPath`, `makeCmdLine`, `createEnvBlock`; no `StartProcess` and no `Exec` (os/exec is a later step). `Token` is here (Go has it in `security_windows.go`, not taken). |
| `types_windows.gp`, `zerrors_windows.gp`, `wtf8_windows.gp` | the same | `types_windows.gp` lacks the socket, DNS and certificate types and constants; `zerrors_windows.gp` (the `Errno` table: `ENOENT` and friends are `APPLICATION_ERROR` + n, with Go's texts) and `wtf8_windows.gp` (file names with unpaired surrogates survive UTF-8 -> UTF-16 -> UTF-8) are the files as they are. |
| `net.gp`, `syscall.gp` | `net.go`, `syscall.go` | `Conn`/`RawConn`, and `ByteSliceFromString`, `StringByteSlice`, `BytePtrFromString`: the files both systems share (split out of `syscall_unix.gp`). |

`Errno.Error()` on Windows asks the system (`FormatMessage`, US English, cached for the two common "not found" numbers) for numbers below
`APPLICATION_ERROR`: `The system cannot find the file specified.`. The Unix-only files carry `//go:build unix` now
(`syscall_unix.gp`, `env_unix.gp`, `dirent.gp`). `Handle` is `uintptr`; pointer arguments go to the system as `uintptr(unsafe.Pointer(p))`
of an object that a reference-counted gocpp pointer keeps alive and that does not move, which is the rule Go's code relies on too.

Not built on Windows: sockets, `StartProcess`, `NewCallback`, `security_windows.go` (SIDs, tokens), the registry.
