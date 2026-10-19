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
  `EINVAL` before any system call (Go's `BytePtrFromString`).
- `wait_linux.gp`: `WaitStatus` (Go's) and `Wait4`.

## Not built

`Syscall`, `Syscall6`, `RawSyscall`, `ForkExec`, `StartProcess`, sockets, `ptrace`, `mmap`, `Select`, `Sysinfo`, `Statfs`, `Flock`,
`Getrlimit`, the `Sockaddr` family, `Exec`, `Chroot`, `Mount`, and everything for other systems. `tools/stubgen`'s
`stubPackage.Partial` says that the package declares only some of Go's names; every name it declares must have Go's type.
Windows is Phase 14 (its own `syscall` surface, with `Errno` values translated from `GetLastError`).
