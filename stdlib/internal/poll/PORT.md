# internal/poll

Step 9.6 (Phase 09). A reduction of go1.25.14's `src/internal/poll` for Linux (BSD-3-Clause, see `../../LICENSE`).

A goroutine is an OS thread (ADR-007), so a read waits in the kernel and there is no netpoller. What `os` needs of the package is
kept with Go's rules: `FD` with its `fdMutex`, the reference count and the closed bit (a closed file is never used again, `Close`
during a `Read` is safe, the last reference closes the descriptor), `Read`/`Pread`/`Write`/`Pwrite` (EINTR retry, `io.EOF`, the
1 GiB cut, short writes), `Fstat`, `Fchmod`, `Fchown`, `Ftruncate`, `Fsync`, `Fchdir`, `Seek`, `ReadDirent`, `Dup`, `RawControl`,
`RawRead`, `RawWrite`, `ErrFileClosing` (`use of closed file`), `ErrNoDeadline`, `ErrDeadlineExceeded`, `ErrNetClosing`,
`ErrNotPollable`.

- `fd_mutex.gp`: the same rules as Go's `fd_mutex.go` with the read and write locks as `sync.Mutex` values and the closed bit and
  the reference count in one `atomic.Uint64` (Go parks waiters on runtime semaphores). A call that waits for a lock while `Close`
  happens gets `ErrFileClosing` once it has the lock.
- `fd_poll.gp`: `pollDesc` has no poller; a wait is `ppoll(2)` on the calling thread (a goroutine is a thread), on the descriptor and
  on a **wake descriptor**, an eventfd that the first wait of each direction makes (`gp::syscall::sys_wake_new`; a file that never
  waits never makes one, and when the process has no descriptor left (EMFILE) the wait goes on **without** it, with `ppoll`
  limited to 25 ms and `closing` and the deadlines looked at again, as Go's wait does not fail for lack of descriptors; and a regular file is never pollable, so its reads stay the one system call they were). `evict` (called
  by `Close`) sets `closing` under the descriptor's mutex and signals the wake descriptor of each direction that has a waiter: the waiting `Read`/`Write` (and every
  later wait) fails with `ErrFileClosing`, which `os` shows as `file already closed`. `SetDeadline`, `SetReadDeadline` and
  `SetWriteDeadline` (`fd_deadline_unix.gp`) store a monotonic deadline per direction and signal the wake descriptor, so a wait in
  progress sees the new deadline; the wait's `ppoll` has the remaining time as its timeout and an expired deadline fails with
  `ErrDeadlineExceeded` (`i/o timeout`); `prepareRead`/`prepareWrite` fail at once when the deadline has passed, even when data
  could be read (Go's rule). A file that is not pollable returns `ErrNoDeadline`. `FD.Close` of a pollable file in non-blocking
  mode waits until the last reference has gone (Go's `csema`; a `Read` woken by `Close` has returned by then), a blocking
  one (`SetBlocking`, `Fd()`) does not. Which files are pollable is decided by `package os` (`newFile`): the ends of `Pipe`, a
  FIFO, a socket or a terminal/character device (not the memory devices `/dev/null`, `/dev/zero`, `/dev/urandom`, ...) opened with
  `Open`, and any descriptor that already was non-blocking; they are put in non-blocking mode (`Pipe` makes them so with
  `pipe2(O_NONBLOCK)`), and `File.Fd` puts them back in blocking mode as in Go, after which `Close` no longer wakes a blocked call
  (also as in Go). Reads of a pipe that has no data cost `read` (EAGAIN), the wake `eventfd` (once) and `ppoll`.
  Windows has no `pollDesc`: `fd_deadline.gp` (tag `windows`) keeps `ErrNoDeadline` there, and `Close` of a pipe cancels the I/O
  (below).
- `copy_file_range_linux.gp`, `copy_file_range_unix.gp`: Go's `CopyFileRange` (for `os.File.ReadFrom`), with
  `unix.CopyFileRange` as the native `gp::syscall::sys_copy_file_range` (no offsets, flags 0; it repeats a call that a
  signal interrupted) and `unix.KernelVersion` as `kernelVersion` (the same parse of `syscall.Uname`'s `Release`).
- Not taken: the socket functions, `sendfile`, `splice`, `writev`, `Accept`, `ReadFrom`/`WriteTo` of sockets.

## Windows (step 14.3, ADR-076)

`fd_windows.gp` is go1.25.14's `fd_windows.go` reduced to what the blocking model needs: `FD` has no `operation`, no overlapped bookkeeping, no
`pollDesc` and no socket code; `Init(net, pollable)` only records the kind (`file`, `console`, `pipe`, `file+net`) and ignores `pollable`; `Read`,
`Write`, `Pread`, `Pwrite`, `Seek`, `Fchmod`, `Fchown`, `Fchdir`, `Fsync`, `Ftruncate`, `GetFileType`, `GetFileInformationByHandle`, `RawRead`,
`RawWrite`, `RawControl` are Go's with a direct `ReadFile`/`WriteFile` (a `*Overlapped` only for the offset of `Pread`/`Pwrite`, as Go does for a
blocking handle, and the file pointer is put back); the console reads and writes (UTF-16 <-> UTF-8, `0x1A`, half surrogates) are Go's code. A write to
the standard output or error handle first flushes C's stdio (`gp::syscall::sys_flush_stdio`: the order rule of the `fmt` shim). **`Close` of a
pipe cancels the I/O that other threads wait for (`CancelIoEx`) and their `Read`/`Write` fail with `ErrFileClosing`** (Go does the same for
its overlapped pipes); `Close` of a file or console handle does not wait for a blocked call, and does not wait for the other references (as on
Linux here). `fd_deadline.gp` (tag `windows`) holds `SetDeadline` and friends (`ErrNoDeadline`); the Unix files carry `//go:build unix`.

