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
- `fd_poll.gp`: `pollDesc` has no poller. `init` succeeds (the descriptor is "pollable") and a wait is `poll(2)` on the calling
  thread, but `package os` calls `init` with pollable=true only for a descriptor that was already non-blocking; all the other
  files stay in blocking mode. `SetDeadline`, `SetReadDeadline` and `SetWriteDeadline` return `ErrNoDeadline`. **`Close` does
  not wake a `Read` or `Write` that is blocked in the kernel** (pipe, FIFO, tty): `evict` does nothing and a wait is not
  interruptible. The fix (plans/TODO.md, "9.6 lows") is a wake channel: an eventfd or self-pipe that `pollDesc.wait` polls
  together with the fd and `evict()` signals.
- Not taken: the socket functions, `sendfile`, `splice`, `copy_file_range`, `writev`, `Accept`, `ReadFrom`/`WriteTo` of sockets.

## Windows (step 14.3, ADR-076)

`fd_windows.gp` is go1.25.14's `fd_windows.go` reduced to what the blocking model needs: `FD` has no `operation`, no overlapped bookkeeping, no
`pollDesc` and no socket code; `Init(net, pollable)` only records the kind (`file`, `console`, `pipe`, `file+net`) and ignores `pollable`; `Read`,
`Write`, `Pread`, `Pwrite`, `Seek`, `Fchmod`, `Fchown`, `Fchdir`, `Fsync`, `Ftruncate`, `GetFileType`, `GetFileInformationByHandle`, `RawRead`,
`RawWrite`, `RawControl` are Go's with a direct `ReadFile`/`WriteFile` (a `*Overlapped` only for the offset of `Pread`/`Pwrite`, as Go does for a
blocking handle, and the file pointer is put back); the console reads and writes (UTF-16 <-> UTF-8, `0x1A`, half surrogates) are Go's code. A write to
the standard output or error handle first flushes C's stdio (`gp::syscall::sys_flush_stdio`: the order rule of the `fmt` shim). **`Close` of a
pipe cancels the I/O that other threads wait for (`CancelIoEx`) and their `Read`/`Write` fail with `ErrFileClosing`** (Go does the same for
its overlapped pipes); `Close` of a file or console handle does not wait for a blocked call, and does not wait for the other references (as on
Linux here). `fd_deadline.gp` holds `SetDeadline` and friends (`ErrNoDeadline`), shared with Linux; the Unix files carry `//go:build unix`.

