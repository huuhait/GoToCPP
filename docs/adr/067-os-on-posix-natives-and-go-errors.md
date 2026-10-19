# ADR-067: Package `os` is Go's own `os` over small POSIX natives, with Go's error types

Status: accepted (2026-10-18)

## Context

Step 9.6 builds `os` before `fmt` (which imports it), `io` (not ported yet: 9.4) and `io/fs`. `tools/portstd` refuses `os`:
it needs assembly, `//go:linkname` into the runtime and `internal/poll`'s netpoller. The plan (Phase 09 §2.1) sketched
"gocpp code that calls `<unistd.h>` through header imports", but the errors of `os` are the hard part to get exactly
right: the `*PathError`/`*LinkError`/`*SyscallError` texts, `syscall.Errno` and its `Is` mapping, `ErrClosed`, `io.EOF` for
reads, the order of the checks in `RemoveAll`, `MkdirAll`, `CreateTemp`. All of that is pure Go code in Go's `os`.

## Decision

1. **Copy Go's `os` for Linux** (the `.go` files renamed `.gp`, Go's names, Go's build constraints) and replace only what
   needs the runtime. `PORT.md` lists each change. Windows (Phase 14) adds `*_windows.gp` files with Go's names.
2. **`syscall` is a subset** (`stdlib/syscall`): Go's `zerrors_linux_amd64.go` (the error and signal tables, so
   `Errno.Error()` is Go's text), `Errno`, `Signal`, `Stat_t`, `Dirent`, ..., and about forty system calls. Each call is a
   `//gocpp:native` function (`gp::syscall::sys_*`, `runtime/include/gocpp/syscall.hpp`, implemented in
   `runtime/src/os/linux/posix_file.cpp`, the only place that includes OS headers) that **returns the errno** of a failure
   and never throws; the `.gp` wrapper builds the `Errno` error. The natives repeat a call that a signal interrupted
   (EINTR) as Go's `ignoringEINTR` does, add `O_CLOEXEC` to every `open`, cut one read or write at 1 GiB like `maxRW`,
   and flush C's stdio before a write to descriptor 1 or 2 (the order rule of the `fmt` shim).
3. **`internal/poll` is reduced to the blocking case**: a goroutine is an OS thread (ADR-007), so there is no netpoller
   (`SetDeadline` is `ErrNoDeadline`; `Close` does not wake a blocked `Read`). The `fdMutex` rules (reference count, closed
   bit, serialized reads and writes, the last reference closes the descriptor) are kept, with `sync.Mutex` for the two locks.
4. **`io`, `io/fs`, `internal/filepathlite` are copies of Go's files** (marked in their `PORT.md`): step 9.4 replaces `io` and
   `io/fs` with `tools/portstd`'s output. The declarations, and so the identity of `io.EOF` and `fs.ErrNotExist`, do not
   change. `io/fs` lacks `Glob` and `Sub` until `path` is ported (`stubPackage.Missing`).
5. **Process state**: `os.Args` and `os.Environ` come from the runtime (`gp::args`, `gp::envs`); the environment is Go's
   copy with one `sync.RWMutex` and `Setenv` also calls `setenv(3)` (C and C++ code the program calls sees it). `os.Exit` is
   `gp::os::exit` (step 8.9).
6. **`fmt.Fprint*(os.Stdout, ...)` goes through the `*os.File` in the variable**, as an `io.Writer` (the `fmt` shim's
   `stdout_`/`stderr_` special case is gone), so assigning `os.Stdout` works. `fmt.Print*` still writes to descriptor 1 until 9.7.
7. **Not built**: `os.Root`/`OpenRoot`, `StartProcess` (ENOSYS), pidfd, `os/exec`, `os/signal`, deadlines, zero-copy
   `ReadFrom`/`WriteTo`, finalizers (an unclosed `*File` keeps its descriptor until the program ends).

## Alternatives

- **`os` as `.gp` calling `<unistd.h>` through header imports** (the plan's third way): the error values would be rebuilt by
  hand (a second source of truth for `Errno` texts and `Is`), and every call would be a C++ function with its own errno
  convention. Rejected: the natives are the same functions with a fixed convention, and the Go code around them is Go's.
- **`os` as a C++ runtime package** (like `fmt`): no way to reuse Go's tests (step 9.10) and the error types would have to be
  described to the type system twice.
- **Port `internal/poll` whole** with a thread-based poller: weeks of work for deadlines on files that Go itself cannot poll.

## Consequences

Good: the error values and their texts are Go's by construction (`tests/run/os_*.gp` compare with real Go); Windows adds
files, not a rewrite; the natives are small and tested by `runtime/test/syscall_test.cpp` (errno per failure, O_CLOEXEC,
EINTR retry through a signal, threads setting the environment).

Bad: `RemoveAll` is Go's portable path-walking version, not the `openat` one, so a tree deeper than about 2000 levels
fails with `lstat ...: file name too long`, `RemoveAll("file/x")` on a regular file returns nil, and permission texts differ
(stdlib/os/PORT.md lists them). `Close` does not wake a `Read` or `Write` blocked on a pipe, FIFO or tty (a wake channel in
`pollDesc` is the follow-up, TODO "9.6 lows"). Compiling a program that imports `os` compiles `os`, `syscall`, `internal/poll`, `io`, `io/fs`, `time` (cached by Phase 06).
An unclosed file leaks its descriptor. `SIGPIPE` on standard output ends the program as Go's does only through the
natives' `die_of_sigpipe` (`fmt` and `os.Stdout.Write` both).
