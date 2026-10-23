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
file_unix.go file_open_unix.go getwd.go path.go path_unix.go pipe2_unix.go proc.go rawconn.go removeall_at.go
removeall_unix.go stat.go stat_linux.go stat_unix.go sys.go sys_linux.go sys_unix.go tempfile.go types.go types_unix.go
wait_waitid.go zero_copy_linux.go zero_copy_posix.go` and `exec_posix.go`'s `ProcessState` (in `process.gp`); for Windows
also `removeall_noat.go` and `zero_copy_stub.go` (below).

## Files written for gocpp

- `process.gp`: `Process`, `ProcAttr`, `Signal`, `Interrupt`, `Kill`, `FindProcess`, `Release`, `Kill`, `Signal`, `Wait`,
  `ProcessState` (Go's `exec.go`, `exec_posix.go`, `exec_unix.go` without the pidfd paths: a Process is a pid, `Wait` is
  Go's `pidWait`: `blockUntilWaitable` (`wait_waitid.gp`, `waitid(2)` with `WNOWAIT`, so a process that is not a child fails
  with `waitid: no child processes` as in Go), then `wait4`). `StartProcess` returns `&PathError{Op: "fork/exec", Err: syscall.ENOSYS}`: fork and exec are not built (`os/exec`
  is a later step).
- `gocpp_helpers.gp`: `sortDirEntries` and `growBytes` (one-line calls of `slices.SortFunc` and `slices.Grow`, since step 7.10),
  `atomic.Pointer[dirInfo]` as a mutex-protected pointer, and `lstatat`.
- `root_unix.gp`: the part of Go's `root_unix.go` that `removeall_at.gp` needs (`sysfdType`, `rootOpenDir`, `removefileat`,
  `removedirat`, `checkSymlink`, `readlinkat`), with `errSymlink` (`root_openat.go`) and `isNoFollowErr` (`eloop_other.go`);
  `unix.Openat`, `Unlinkat` and `Readlinkat` are the natives `gp::syscall::sys_openat`, `sys_unlinkat` and `sys_readlinkat`.
  `os.Root` itself is not built (below).

## Changes to Go's files

- `runtime.KeepAlive` and `runtime.SetFinalizer` calls are dropped (memory is reference counted; a `*File` that is dropped does
  not close its descriptor, as a Go `*File` that is collected would: gocpp has no finalizers, so **an unclosed file leaks its
  descriptor** until the program ends).
- `internal/poll` is gocpp's reduction (stdlib/internal/poll/PORT.md): `newFile` marks a descriptor "pollable" when it is a `Pipe`
  end, a FIFO, a socket or a terminal that `Open` opened, or already is non-blocking (a pipe made with `O_NONBLOCK`, a shared
  standard stream): it is put in non-blocking mode and an EAGAIN is waited out with `ppoll(2)` on the calling thread together with a
  wake descriptor. A regular file, a directory, a block device and `/dev/null` and the other memory devices stay in blocking mode
  (one `fstat` at `Open` decides). **`Close` wakes a `Read` or `Write` that is blocked on a pollable file** with `ErrClosed`, and
  `SetDeadline`, `SetReadDeadline` and `SetWriteDeadline` work on them (`ErrNoDeadline` for the others), as in Go. `File.Fd()`
  puts a pollable file back in blocking mode (also Go's rule); a `Close` still wakes a call that waits in `ppoll`, but not one
  that is already inside a blocking kernel `read` or `write` (a call that started after `Fd()`). `os.Stdin`/`Stdout`/`Stderr`
  are not pollable unless the process got them non-blocking. Without a free descriptor for the wake eventfd (EMFILE) a wait
  goes on without it and polls its state every 25 ms (`Close` and deadlines still work, a little late).
- `file_posix.go`: `File.Close` calls `noteClose` (gocpp_helpers.gp), which records that the original `os.Stdout` or `os.Stdin` is
  closed. With the init of `gocpp_helpers.gp` (`setFmtStdout`, `setFmtStdin`, natives `gp::fmt::set_os_stdout` and `set_os_stdin`)
  it lets package fmt follow the variables: `fmt.Print`, `Println` and `Printf` write to the file that `os.Stdout` holds, and
  `Scan`, `Scanln` and `Scanf` read the file that `os.Stdin` holds, once the program has assigned another file (or copied one
  over it, `*os.Stdout = *w`) or closed the original; until then fmt uses descriptors 1 and 0 itself ([ADR-100](../../docs/adr/100-fmt-stays-native-decision-9-7-a.md)).
- `internal/syscall/unix` is not ported: `Fcntl(F_GETFL)` is the native `sysFcntlGetFl`, `UTIME_OMIT` a constant.
- `internal/testlog`, `internal/filepathlite`, `internal/itoa`, `internal/oserror`: used as in Go (shims of step 9.2, and
  `internal/filepathlite` ported in this step).
- `proc.go`: `runtime_args` is a native (`gp::syscall::runtime_args`, the program's `argv`); `runtime_beforeExit` is dropped
  (no race detector, no coverage).
- `tempfile.go`: `runtime_rand` is a native (`gp::syscall::runtime_rand`, a per-thread xorshift seeded from the clock, the thread
  and a counter).
- `file_unix.go`: `net_newUnixFile` (the linkname for package net) is dropped; `sigpipe` is the native `gp::syscall::sys_sigpipe`
  (`gp::os::die_of_sigpipe`).
- `removeall_at.go` is Go's (`RemoveAll` relative to open directories with `openat`/`unlinkat`, so a tree deeper than
  `PATH_MAX` goes and the errors say `unlinkat` and `openfdat` as in Go), with the build line `unix` (Go's adds `wasip1` and
  `windows`: the Windows build here keeps `removeall_noat.go`, below).
- `zero_copy_linux.go`: `File.ReadFrom` tries `copy_file_range(2)` first (`internal/poll.CopyFileRange`, kernel 5.3 or later),
  so its errors carry the `copy_file_range:` prefix as in Go; the `sendfile` path of `WriteTo` and the `splice` path of
  `ReadFrom` need a socket of package `net` (`getPollFDAndNetwork`), which gocpp does not have, so they are "not handled"
  (the generic `io.Copy`).
- `dir.go`: `CopyFS` uses `io/fs.WalkDir` (ported); `ReadDir` sorts with `sortDirEntries`.
- `Root`, `OpenRoot` and `OpenInRoot` (Go 1.24's `os.Root`) are **not built** (`root*.go` need `openat2` and the `*at`
  system calls); `stubPackage.Missing` in `tools/stubgen` lists them.
- `os.Stdout`, `os.Stderr` and `os.Stdin` are `NewFile` values as in Go. The native `fmt` engine follows reassignment of
  `os.Stdout` and `os.Stdin` through the registered hooks described above; it was not replaced by a Go-source port
  ([ADR-100](../../docs/adr/100-fmt-stays-native-decision-9-7-a.md)).

## Findings

- `EMFILE` (too many open files) does not come at Go's count. A program that only opens files gets it about 3 descriptors
  later than the same Go program: Go 1.25's netpoller makes an epoll descriptor and an eventfd at the first `Open`, and its
  runtime keeps the cgroup's `cpu.max` file open (for the `GOMAXPROCS` updates) where there is one; gocpp has neither (a
  goroutine is a thread, ADR-007). A program that waits on pipes or terminals gets it earlier than Go: `internal/poll` makes
  one wake eventfd per waiting direction per descriptor (stdlib/internal/poll/PORT.md), where Go's netpoller shares its one
  epoll descriptor (10 pipes blocked in `Read` under `ulimit -n 64`: Go opens 38 files, gocpp 31). Kept on purpose: holding
  descriptors that do nothing only to fail at Go's count would waste them.
- Writes to descriptors 1 and 2 flush C's stdio first (`gp::detail::flush_c_stdio`), so what C or C++ code printf'd comes out
  before, as for the `fmt` shim and the builtin `print`.
- `os.Exit` is `syscall.Exit`, which is `gp::os::exit` (step 8.9): C stdio is flushed, no deferred function runs, no static
  destructor, the ASan leak check runs first.

## Windows (step 14.3, ADR-076)

For `windows/amd64` the package is the same shared files (`dir.gp env.gp error.gp error_errno.gp executable.gp file.gp file_posix.gp getwd.gp
path.gp proc.gp rawconn.gp removeall_noat.gp stat.gp sys.gp tempfile.gp types.gp zero_copy_posix.gp zero_copy_stub.gp gocpp_helpers.gp`,
which carry no build line, or one that holds on Windows: `removeall_noat.gp` and `zero_copy_stub.gp` are built for Windows only) and Go's own Windows files, selected by their `_windows` names: `dir_windows.gp executable_windows.gp file_windows.gp
path_windows.gp stat_windows.gp sys_windows.gp types_windows.gp` (go1.25.14, `src/os`). `stat_windows`, `types_windows`, `path_windows`,
`sys_windows` and `executable_windows` are Go's files byte for byte; the other two have these edits (the same rules as the Linux files):

- `file_windows.gp`: no `runtime.SetFinalizer`/`KeepAlive` (an unclosed `*File` keeps its handle until the program ends); `File.dirinfo` is the
  mutex-protected `dirInfoPointer` of `gocpp_helpers.gp`; `newFile` does not ask `windows.IsNonblock` (**a handle opened with
  `FILE_FLAG_OVERLAPPED` that is given to `NewFile` is used with synchronous calls, which the system refuses**: every handle that package os
  opens is synchronous, `syscall.Open`'s and `CreatePipe`'s are); `net_newWindowsFile` (for package net) is gone.
- `dir_windows.gp`: no `runtime.KeepAlive`.

Not taken: `root_windows.go` and the `Root` family (`stubPackage.Missing`, as on Linux), `removeall_at.go`/`removeall_windows.go` (the
`NtCreateFile` based `RemoveAll`; `removeall_noat.gp`, the path walking version, serves, with Windows' own error texts: a tree deeper
than the path limit fails and stays where it stopped, `RemoveAll("file\\x")` with `file` a regular file returns nil, and the permission errors
say `remove d\f` and `open d` where Go says `unlinkat` and `openfdat`), `exec.go`/`exec_windows.go`/`exec_posix.go`, written again as `process_windows.gp`: a `Process` that owns the
handle that `FindProcess` opened (`OpenProcess`), `Kill` is `TerminateProcess`, `Wait` is `WaitForSingleObject` + `GetExitCodeProcess`, `Release` returns
`EINVAL` the second time, `ProcessState.String` prints `exit status 0xc0000005` for large codes; `StartProcess` fails with `ENOSYS` (os/exec comes
later). `os.Args` is the runtime's (Go's `init` splits `GetCommandLine`; the runtime did it at the start with the same rules, ADR-053).
`os.Environ`, `Getenv`, `Setenv` (case-insensitive names, `=C:` entries) are the system's block through `syscall`.

What the tests found about Windows that a Linux programmer would not expect (all the same as real Go's): `Remove` of an open file fails
(sharing violation), `Rename` over an open file is `Access is denied`, names are case-insensitive and lose trailing dots, `a:b` makes an NTFS stream,
`Chmod` is the read-only attribute, `Mode` is `-rw-rw-rw-`/`-r--r--r--`/`drwxrwxrwx`, `Getuid` is -1, `SameFile` compares volume and file index,
`Close` of a pipe wakes a blocked `Read` (`CancelIoEx`; Linux pollable pipes use the wake descriptor described above), and a program without the long-path manifest cannot
`Chdir` into a path of more than 260 characters (Go's linker writes the manifest, gocpp's driver does not yet: step 14.2).

Differences from Go 1.25.14 that remain (`tests/run/os_win_*.gp` compare with Go 1.25.14 built for Windows and run on the same PC): `CanUseLongPaths` is
false, so package os adds `\\?\` to long paths itself (`fixLongPath`); `NewFile` of an overlapped pipe is not supported; no finalizers. The console
paths (`readConsole`/`writeConsole`, Go's code) run in `TestWindowsConsole` (`tests/windows_programs_test.go`, a pseudo console over `ssh -tt`
through `internal/winssh.RunConsole`): UTF-8 through `fmt` and `os.Stdout`, `Stat` says character device, and a console read gives UTF-8.
