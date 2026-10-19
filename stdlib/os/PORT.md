# os

Step 9.6 (Phase 09). Not made by `tools/portstd`: `portstd` refuses `os` (it needs assembly, `//go:linkname` into the runtime
and `internal/poll`'s netpoller). The files are **copies of go1.25.14's `src/os`** for Linux (BSD-3-Clause, see `../LICENSE`)
with the edits below, so the error values, the texts of the errors and the edge cases are Go's own. Windows (Phase 14)
adds `*_windows.gp` files with Go's names; the `//go:build` lines and the `_unix`/`_linux` names are kept for that.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` |
| System | linux (amd64 and arm64: the Linux files of `os` are the same) |
| Natives | `syscall` (stdlib/syscall/PORT.md) and `gp::syscall::runtime_args`, `runtime_rand`, `sys_sigpipe`, `sys_fcntl_getfl` |

## Files taken (Go's names, `.go` -> `.gp`)

`dir.go dir_unix.go dirent_linux.go env.go error.go error_errno.go executable.go executable_procfs.go file.go file_posix.go
file_unix.go file_open_unix.go getwd.go path.go path_unix.go pipe2_unix.go proc.go rawconn.go removeall_noat.go stat.go
stat_linux.go stat_unix.go sys.go sys_linux.go sys_unix.go tempfile.go types.go types_unix.go` and `exec_posix.go`'s
`ProcessState` (in `process.gp`).

## Files written for gocpp

- `process.gp`: `Process`, `ProcAttr`, `Signal`, `Interrupt`, `Kill`, `FindProcess`, `Release`, `Kill`, `Signal`, `Wait`,
  `ProcessState` (Go's `exec.go`, `exec_posix.go`, `exec_unix.go` without the pidfd paths: a Process is a pid, `Wait` is
  `wait4`). `StartProcess` returns `&PathError{Op: "fork/exec", Err: syscall.ENOSYS}`: fork and exec are not built (`os/exec`
  is a later step).
- `gocpp_helpers.gp`: stand-ins for generic library functions that the ports do not have yet (`slices.SortFunc`, `slices.Grow`,
  `atomic.Pointer[dirInfo]` as a mutex-protected pointer), `lstatat`, and `readFrom`/`writeTo` (always "not handled": Go's
  copy_file_range/sendfile/splice paths are not built, so `File.ReadFrom` and `WriteTo` use `io.Copy`).

## Changes to Go's files

- `runtime.KeepAlive` and `runtime.SetFinalizer` calls are dropped (memory is reference counted; a `*File` that is dropped does
  not close its descriptor, as a Go `*File` that is collected would: gocpp has no finalizers, so **an unclosed file leaks its
  descriptor** until the program ends).
- `internal/poll` is gocpp's blocking reduction (stdlib/internal/poll/PORT.md): `newFile` marks a descriptor
  "pollable" only when it already is non-blocking (a pipe made with `O_NONBLOCK`, a shared standard stream): `pollDesc.init`
  then succeeds and an EAGAIN is waited out with `poll(2)` on the calling thread. Every file that package os opens itself stays
  in blocking mode. `SetDeadline` and friends return `ErrNoDeadline`.
- **`Close` does not wake a `Read` or `Write` that is blocked in the kernel** on a pipe, a FIFO or the tty: `Close` on a pipe
  whose reader is blocked returns nil at once and the reader stays blocked until the writer closes or writes (the same for a
  `Write` blocked on a full pipe). Go's poller wakes them with `ErrClosed`. A program that closes the read end to stop a reader
  goroutine hangs. The follow-up is in plans/TODO.md ("9.6 lows", priority).
- `internal/syscall/unix` is not ported: `Fcntl(F_GETFL)` is the native `sysFcntlGetFl`, `UTIME_OMIT` a constant.
- `internal/testlog`, `internal/filepathlite`, `internal/itoa`, `internal/oserror`: used as in Go (shims of step 9.2, and
  `internal/filepathlite` ported in this step).
- `proc.go`: `runtime_args` is a native (`gp::syscall::runtime_args`, the program's `argv`); `runtime_beforeExit` is dropped
  (no race detector, no coverage).
- `tempfile.go`: `runtime_rand` is a native (`gp::syscall::runtime_rand`, a per-thread xorshift seeded from the clock, the thread
  and a counter).
- `file_unix.go`: `net_newUnixFile` (the linkname for package net) is dropped; `sigpipe` is the native `gp::syscall::sys_sigpipe`
  (`gp::os::die_of_sigpipe`).
- `removeall_noat.go` is used (Go's Linux build uses `removeall_at.go`, the `openat`-based version). The results differ in
  these cases (all found by the review of step 9.6):
  - a tree deeper than about 2000 levels: this version walks paths, so it fails with `lstat ...: file name too long` and
    leaves the tree where it stopped; Go's `openat` version succeeds;
  - `RemoveAll("file/x")` where `file` is a regular file returns nil here; Go returns `unlinkat file/x: not a directory`;
  - the texts of permission errors: Go says `unlinkat ./d/f: permission denied` and `openfdat ./d: permission denied`, this
    version `remove d/f: permission denied` and `open d: permission denied`;
  - `RemoveAll("..")`-like edge cases: `remove ..: directory not empty` here, `unlinkat ./..: directory not empty` in Go.
- `dir.go`: `CopyFS` uses `io/fs.WalkDir` (ported); `ReadDir` sorts with `sortDirEntries`.
- `Root`, `OpenRoot` and `OpenInRoot` (Go 1.24's `os.Root`) are **not built** (`root*.go` need `openat2` and the `*at`
  system calls); `stubPackage.Missing` in `tools/stubgen` lists them.
- `os.Stdout`, `os.Stderr` and `os.Stdin` are `NewFile` values as in Go. `fmt.Print*` still writes to descriptor 1 directly
  (the `fmt` shim, until step 9.7 ports Go's `fmt`), so assigning `os.Stdout` redirects `fmt.Fprint*(os.Stdout, ...)` and not
  `fmt.Println`.

## Findings

- Writes to descriptors 1 and 2 flush C's stdio first (`gp::detail::flush_c_stdio`), so what C or C++ code printf'd comes out
  before, as for the `fmt` shim and the builtin `print`.
- `os.Exit` is `syscall.Exit`, which is `gp::os::exit` (step 8.9): C stdio is flushed, no deferred function runs, no static
  destructor, the ASan leak check runs first.
