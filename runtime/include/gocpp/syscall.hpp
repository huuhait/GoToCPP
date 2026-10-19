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

// sys_poll waits until fd can be read (write false) or written (write true), or has an error or a
// hangup (poll(2), without a timeout; EINTR is retried). It returns 0 or the errno.
int64_t sys_poll(int64_t fd, bool write);
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

// sys_selftest_retry is a test seam for the EINTR rule (runtime/test/syscall_test.cpp): it makes a
// pipe, installs a SIGALRM handler without SA_RESTART that writes one byte into the pipe, arms a
// timer, and calls sys_read on the empty pipe. The signal interrupts the read (EINTR); a sys_read
// that repeats the call gets the byte on the second try. It returns (handler runs, errno of the
// read) and restores the signal's old action: (n >= 1, 0) when the retry works.
std::tuple<int64_t, int64_t> sys_selftest_retry();

// sys_nodename is the host name (uname(2)'s nodename).
std::tuple<gp::string, int64_t> sys_nodename();

// sys_exit is gp::os::exit (flush, leak check in an ASan build, _exit): no deferred function runs.
[[noreturn]] void sys_exit(int64_t code);

} // namespace gp::syscall
