# ADR-067: Package `os` is Go's own `os` over small POSIX natives, with Go's error types

Status: accepted (2026-10-18; point 3 and the `Close` sentence under Bad amended 2026-10-19: `Close` wakes a blocked `Read` or `Write`, and deadlines work)

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
   **Amended 2026-10-19 (follow-up of 9.6, job 09-pollclose): the reduction is no longer "blocking only".** A pipe end, a
   FIFO, a socket or a terminal that `Open` opened, and a descriptor that already was non-blocking, are *pollable*: they are
   in non-blocking mode and an EAGAIN is waited out by `ppoll(2)` on the descriptor and on a wake eventfd that `Close`
   (`pollDesc.evict`) and `SetDeadline` signal. So `Close` wakes a `Read` or `Write` that waits with `ErrClosed`, and
   `SetDeadline`, `SetReadDeadline` and `SetWriteDeadline` work on those files (`ErrNoDeadline` for the others), as in Go.
   Regular files, directories and the memory devices stay in blocking mode, with the one system call per read or write
   they had. A wait that cannot make its eventfd (EMFILE) goes on without it and looks at the file's state every 25 ms,
   as Go's wait does not fail for lack of descriptors. A call that is blocked inside a kernel `read` (a file after
   `Fd()`, which returns blocking mode as in Go) is still not woken. Details: stdlib/internal/poll/PORT.md.
4. **`io`, `io/fs`, `internal/filepathlite` are copies of Go's files** (marked in their `PORT.md`): step 9.4 replaces `io` and
   `io/fs` with `tools/portstd`'s output. The declarations, and so the identity of `io.EOF` and `fs.ErrNotExist`, do not
   change. `io/fs` lacks `Glob` and `Sub` until `path` is ported (`stubPackage.Missing`).
5. **Process state**: `os.Args` and `os.Environ` come from the runtime (`gp::args`, `gp::envs`); the environment is Go's
   copy with one `sync.RWMutex` and `Setenv` also calls `setenv(3)` (C and C++ code the program calls sees it). `os.Exit` is
   `gp::os::exit` (step 8.9).
6. **`fmt.Fprint*(os.Stdout, ...)` goes through the `*os.File` in the variable**, as an `io.Writer` (the `fmt` shim's
   `stdout_`/`stderr_` special case is gone), so assigning `os.Stdout` works. `fmt.Print*` still writes to descriptor 1 until 9.7.
7. **Not built**: `os.Root`/`OpenRoot`, `StartProcess` (ENOSYS), pidfd, `os/exec`, `os/signal`, deadlines, zero-copy
   `ReadFrom`/`WriteTo`, finalizers (an unclosed `*File` keeps its descriptor until the program ends). (Superseded in part
   2026-10-23, job dev.syscalldiffs: `ReadFrom` tries `copy_file_range` as Go does; `sendfile` and `splice` still wait for
   package `net`.)

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
(stdlib/os/PORT.md lists them). (Superseded 2026-10-23, job dev.syscalldiffs: on Linux `RemoveAll` is Go's
`removeall_at.go` over `openat`/`unlinkat` natives; only Windows keeps the path-walking version.) (Amended 2026-10-19: `Close` now wakes a `Read` or `Write` blocked on a pipe, FIFO or tty, see point 3.) Compiling a program that imports `os` compiles `os`, `syscall`, `internal/poll`, `io`, `io/fs`, `time` (cached by Phase 06).
An unclosed file leaks its descriptor. `SIGPIPE` on standard output ends the program as Go's does only through the
natives' `die_of_sigpipe` (`fmt` and `os.Stdout.Write` both).
