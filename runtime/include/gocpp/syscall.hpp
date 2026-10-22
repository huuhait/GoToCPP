// gocpp/syscall.hpp: the native functions of the standard package syscall (Phase 09, step 9.6;
// ADR-067).
//
// Go's package syscall is written in .gp (stdlib/syscall) except for the system calls themselves,
// which are these functions, named by //gocpp:native lines (ADR-027):
//
//     //gocpp:native gp::syscall::sys_read
//     func sysRead(fd int, p []byte) (n int, errno int)   // ::gp::syscall::sys_read(fd, p)
//
// The C++ types are those of the Go parameters (int is int64_t, a string is gp::string, a slice
// gp::slice<T>, several results a std::tuple). Every function that can fail returns the **errno**
// of the failure (0 for success; the .gp wrapper makes a syscall.Errno of it, whose texts are
// Go's; a call that also returns a value returns {-1, errno} or {0, errno} for it, the value
// meaning nothing then), never an exception, and none of them touches errno of the caller. A call
// that a signal interrupted (EINTR) is repeated here, before the Go code sees it, as Go's os
// repeats it (ignoringEINTR): this is where a descriptor's read, write, open, stat and friends are
// retried; close is not (the descriptor is gone whatever the result).
//
// A string argument is a path or a name: it holds no NUL byte (the wrapper in .gp checked: a path
// with one is EINVAL, as in Go) and is copied to a NUL-terminated buffer here. Reads and writes
// work on the bytes of the slice they get, from its start to its length.
//
// The functions are implemented by the OS layer of the runtime (src/os/<os>/posix_file.cpp for
// Linux), the only place that calls the system; this header is standard C++ only. A system with a
// different set of calls (Windows, Phase 14) implements the same functions with its own, and
// translates its errors to the Linux errno numbers that Go's syscall.Errno table knows.
#pragma once

#include <gocpp/slice.hpp>
#include <gocpp/string.hpp>

#include <cstdint>
#include <tuple>

namespace gp::syscall {

// --- the process's environment and arguments ---

// runtime_envs is the environment at the start of the program as "KEY=value" strings (gp::envs).
gp::slice<gp::string> runtime_envs();

// runtime_args is the program's arguments, the program name first (gp::args), for os.Args.
gp::slice<gp::string> runtime_args();

// runtime_rand is a random 64-bit number from the runtime's generator (os.CreateTemp names).
uint64_t runtime_rand();

// runtime_setenv and runtime_unsetenv keep the C library's environment the same as the one
// package syscall keeps (setenv(3) and unsetenv(3)), so that C and C++ code the program calls sees
// what os.Setenv set. They ignore a key that the OS refuses.
void runtime_setenv(const gp::string& key, const gp::string& value);
void runtime_unsetenv(const gp::string& key);

// --- descriptors ---

// open is open(2) with O_CLOEXEC added when flags says nothing about it (Go's rule: every
// descriptor that Go opens is close-on-exec). It returns (fd, 0) or (-1, errno).
std::tuple<int64_t, int64_t> sys_open(const gp::string& path, int64_t flags, int64_t perm);
int64_t sys_close(int64_t fd);

// sys_read, sys_write, sys_pread and sys_pwrite are the system calls of the same names, called once
// (one read or write, as Go's syscall.Read and Write are): they return the count of bytes or
// (0, errno); a count of 0 from sys_read is the end of the file. A zero-length slice is a call with
// length 0 (the kernel's answer), not skipped. The length is cut to 1<<30 bytes.
std::tuple<int64_t, int64_t> sys_read(int64_t fd, const gp::slice<uint8_t>& p);
std::tuple<int64_t, int64_t> sys_write(int64_t fd, const gp::slice<uint8_t>& p);
std::tuple<int64_t, int64_t> sys_pread(int64_t fd, const gp::slice<uint8_t>& p, int64_t offset);
std::tuple<int64_t, int64_t> sys_pwrite(int64_t fd, const gp::slice<uint8_t>& p, int64_t offset);
std::tuple<int64_t, int64_t> sys_seek(int64_t fd, int64_t offset, int64_t whence);

// The stat functions fill out (at least 16 elements) with: dev, ino, nlink, mode (the st_mode
// bits), uid, gid, rdev, size, blksize, blocks, atime sec, atime nsec, mtime sec, mtime nsec,
// ctime sec, ctime nsec.
int64_t sys_fstat(int64_t fd, const gp::slice<uint64_t>& out);
int64_t sys_stat(const gp::string& path, const gp::slice<uint64_t>& out);
int64_t sys_lstat(const gp::string& path, const gp::slice<uint64_t>& out);

int64_t sys_ftruncate(int64_t fd, int64_t length);
int64_t sys_truncate(const gp::string& path, int64_t length);
int64_t sys_fsync(int64_t fd);
int64_t sys_fchmod(int64_t fd, int64_t mode);
int64_t sys_chmod(const gp::string& path, int64_t mode);
int64_t sys_fchown(int64_t fd, int64_t uid, int64_t gid);
int64_t sys_chown(const gp::string& path, int64_t uid, int64_t gid);
int64_t sys_lchown(const gp::string& path, int64_t uid, int64_t gid);

// --- names ---

int64_t sys_mkdir(const gp::string& path, int64_t mode);
// sys_getrlimit returns the soft and the hard limit of a resource (RLIM_INFINITY is -1 as an
// int64_t), sys_setrlimit sets both.
std::tuple<int64_t, int64_t, int64_t> sys_getrlimit(int64_t resource);
int64_t sys_setrlimit(int64_t resource, int64_t cur, int64_t max);
int64_t sys_mknod(const gp::string& path, int64_t mode, int64_t dev);
int64_t sys_rmdir(const gp::string& path);
int64_t sys_unlink(const gp::string& path);
int64_t sys_rename(const gp::string& from, const gp::string& to);
int64_t sys_link(const gp::string& oldpath, const gp::string& newpath);
int64_t sys_symlink(const gp::string& oldpath, const gp::string& newpath);

// sys_readlink returns the bytes of the link in buf (no NUL, not cut at the end: n == len(buf)
// may mean that the target is longer).
std::tuple<int64_t, int64_t> sys_readlink(const gp::string& path, const gp::slice<uint8_t>& buf);
int64_t sys_chdir(const gp::string& path);
int64_t sys_fchdir(int64_t fd);

// sys_getcwd writes the current directory and a NUL byte into buf and returns the byte count
// including the NUL (Linux's getcwd system call).
std::tuple<int64_t, int64_t> sys_getcwd(const gp::slice<uint8_t>& buf);

// sys_pipe2 makes a pipe: p[0] is the read end, p[1] the write end (O_CLOEXEC is in flags if
// the caller says so).
int64_t sys_pipe2(const gp::slice<int64_t>& p, int64_t flags);

// sys_getdents reads directory entries into buf in the kernel's linux_dirent64 format, as Go's
// ReadDirent does, and returns the byte count (0 at the end of the directory).
std::tuple<int64_t, int64_t> sys_getdents(int64_t fd, const gp::slice<uint8_t>& buf);

// sys_utimensat sets the access and modification times of path; a time whose seconds and
// nanoseconds are both (1<<30)-2 (UTIME_OMIT) is left alone.
int64_t sys_utimensat(const gp::string& path, int64_t asec, int64_t ansec, int64_t msec,
                      int64_t mnsec);

std::tuple<int64_t, int64_t> sys_dup(int64_t fd);

// The wake descriptor of a pollDesc (internal/poll): an eventfd that Close and SetDeadline
// signal to end a wait in sys_poll_wait. sys_wake_new returns it (non-blocking, close-on-exec),
// sys_wake_signal makes it readable (level triggered: it stays readable) and sys_wake_clear
// empties it.
std::tuple<int64_t, int64_t> sys_wake_new();
void sys_wake_signal(int64_t fd);
void sys_wake_clear(int64_t fd);

// What sys_poll_wait reports.
constexpr int64_t poll_ready = 0;   // fd can be read (or written), or has an error or a hangup
constexpr int64_t poll_woken = 1;   // wakefd is readable
constexpr int64_t poll_timeout = 2; // timeout_ns passed

// sys_poll_wait blocks the calling thread until fd can be read (write false) or written (write
// true), or has an error or a hangup, or wakefd is readable, or timeout_ns nanoseconds passed
// (negative: no timeout); ppoll(2) with EINTR retried. It returns the state and the errno.
std::tuple<int64_t, int64_t> sys_poll_wait(int64_t fd, int64_t wakefd, bool write,
                                           int64_t timeout_ns);
int64_t sys_dup3(int64_t oldfd, int64_t newfd, int64_t flags);
int64_t sys_umask(int64_t mask);
int64_t sys_set_nonblock(int64_t fd, bool nonblocking);
int64_t sys_set_cloexec(int64_t fd);

// --- the process ---

int64_t sys_getpid();
int64_t sys_getppid();
int64_t sys_getuid();
int64_t sys_geteuid();
int64_t sys_getgid();
int64_t sys_getegid();
int64_t sys_getpagesize();

// sys_getgroups fills out and returns the count of groups (when out is empty, the count only).
std::tuple<int64_t, int64_t> sys_getgroups(const gp::slice<int64_t>& out);
int64_t sys_kill(int64_t pid, int64_t sig);

// sys_wait4 is wait4(2) (the options as Linux has them, WNOHANG = 1): out gets the wait status,
// then the 18 numbers of struct rusage (utime sec, usec, stime sec, usec, maxrss, ixrss, idrss,
// isrss, minflt, majflt, nswap, inblock, oublock, msgsnd, msgrcv, nsignals, nvcsw, nivcsw).
std::tuple<int64_t, int64_t> sys_wait4(int64_t pid, int64_t options, const gp::slice<int64_t>& out);

// sys_fcntl_getfl returns the file status flags of fd (F_GETFL).
std::tuple<int64_t, int64_t> sys_fcntl_getfl(int64_t fd);

// sys_sigpipe ends the program with SIGPIPE (gp::os::die_of_sigpipe).
[[noreturn]] void sys_sigpipe();

#ifdef GOCPP_RUNTIME_TEST
// The test seams exist only in the runtime's own test build (GOCPP_RUNTIME_TEST, defined by
// runtime/cxx_test.go): the library that programs link does not have them.

// sys_selftest_retry is a test seam for the EINTR rule (runtime/test/syscall_test.cpp): it makes a
// pipe, installs a SIGALRM handler without SA_RESTART that writes one byte into the pipe, arms a
// timer, and calls sys_read on the empty pipe. The signal interrupts the read (EINTR); a sys_read
// that repeats the call gets the byte on the second try. It returns (handler runs, errno of the
// read) and restores the signal's old action: (n >= 1, 0) when the retry works.
std::tuple<int64_t, int64_t> sys_selftest_retry();

// sys_selftest_poll_eintr is a test seam for the EINTR rule of sys_poll_wait: it waits 150 ms on an
// idle pipe while SIGALRM arrives every 20 ms (no SA_RESTART), and returns the number of signals
// that ran, the state sys_poll_wait reported (or its negated errno) and the milliseconds it took.
std::tuple<int64_t, int64_t, int64_t> sys_selftest_poll_eintr();
#endif // GOCPP_RUNTIME_TEST

// sys_nodename is the host name (uname(2)'s nodename).
std::tuple<gp::string, int64_t> sys_nodename();

// sys_exit is gp::os::exit (flush, leak check in an ASan build, _exit): no deferred function runs.
[[noreturn]] void sys_exit(int64_t code);

// --- Windows (Phase 14, step 14.3; ADR-076; implemented in src/os/windows/win_syscall.cpp) ---
//
// Go's package syscall on Windows reaches the system through Syscall and LoadLibrary, which Go's
// runtime provides; stdlib/syscall/dll_windows.gp takes them from these natives, and everything
// above them (the system calls of syscall, internal/syscall/windows, internal/poll, os) is Go's own
// code. Declared for every system (standard C++ only), defined on Windows only.

// win_syscall15 calls the function at address trap with nargs integer arguments (a1..a15, the rest
// is ignored) on the calling thread, with the Windows x64 calling convention, and returns (RAX,
// 0, GetLastError() read right after the call: a double that the function returns in XMM0 is lost).
// The pointers in the arguments are addresses of memory the caller keeps alive (a gocpp object does
// not move).
std::tuple<uintptr_t, uintptr_t, uintptr_t>
win_syscall15(uintptr_t trap, uintptr_t nargs, uintptr_t a1, uintptr_t a2, uintptr_t a3,
              uintptr_t a4, uintptr_t a5, uintptr_t a6, uintptr_t a7, uintptr_t a8, uintptr_t a9,
              uintptr_t a10, uintptr_t a11, uintptr_t a12, uintptr_t a13, uintptr_t a14,
              uintptr_t a15);

// win_syscall_n is win_syscall15 for up to 42 arguments (Go's limit), given as a slice; the second
// result is 0 (a function that returns a double leaves it in XMM0, which C++ cannot read here).
std::tuple<uintptr_t, uintptr_t, uintptr_t> win_syscall_n(uintptr_t trap,
                                                          const gp::slice<uintptr_t>& args);

// win_loadlibrary is LoadLibraryW(name); win_loadsystemlibrary is LoadLibraryExW(name, 0,
// LOAD_LIBRARY_SEARCH_SYSTEM32): they return (HMODULE, 0) or (0, GetLastError()). name is the
// address of a NUL-terminated UTF-16 string. win_getprocaddress is GetProcAddress(handle, name)
// with the address of a NUL-terminated ASCII name: (address, 0) or (0, GetLastError()).
std::tuple<uintptr_t, uintptr_t> win_loadlibrary(uintptr_t name);
std::tuple<uintptr_t, uintptr_t> win_loadsystemlibrary(uintptr_t name);
std::tuple<uintptr_t, uintptr_t> win_getprocaddress(uintptr_t handle, uintptr_t name);

// win_setenv and win_unsetenv put the variable into the C library's copy of the environment too
// (and keep an empty value in the system's block); key and value are the addresses of
// NUL-terminated UTF-16 strings, the ones that Go's Setenv already made, so unpaired surrogates
// stay as they are.
void win_setenv(uintptr_t key, uintptr_t value);
void win_unsetenv(uintptr_t key);

// sys_flush_stdio flushes C's stdio before a write to a standard handle (flush_c_stdio).
void sys_flush_stdio();

} // namespace gp::syscall
