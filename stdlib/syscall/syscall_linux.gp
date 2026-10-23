// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The system calls of package syscall (Linux). Go's package has them in assembly stubs
// (zsyscall_linux_amd64.go); here each one is a native function of the runtime's OS layer
// (runtime/src/os/linux/posix_file.cpp, declared in gocpp/syscall.hpp) that returns the
// errno of a failure (0 for success) and retries a call that a signal interrupted (EINTR),
// as Go's os does around its system calls. The exported functions turn the errno into an
// [Errno] error.

package syscall

//gocpp:native gp::syscall::sys_open
func sysOpen(path string, flags int, perm int) (fd int, errno int)

//gocpp:native gp::syscall::sys_close
func sysClose(fd int) (errno int)

//gocpp:native gp::syscall::sys_read
func sysRead(fd int, p []byte) (n int, errno int)

//gocpp:native gp::syscall::sys_write
func sysWrite(fd int, p []byte) (n int, errno int)

//gocpp:native gp::syscall::sys_pread
func sysPread(fd int, p []byte, offset int64) (n int, errno int)

//gocpp:native gp::syscall::sys_pwrite
func sysPwrite(fd int, p []byte, offset int64) (n int, errno int)

//gocpp:native gp::syscall::sys_seek
func sysSeek(fd int, offset int64, whence int) (off int64, errno int)

// The stat functions fill out with the 16 numbers of a struct stat: dev, ino, nlink, mode,
// uid, gid, rdev, size, blksize, blocks, then the seconds and nanoseconds of the access,
// modification and status change times.

//gocpp:native gp::syscall::sys_fstat
func sysFstat(fd int, out []uint64) (errno int)

//gocpp:native gp::syscall::sys_stat
func sysStat(path string, out []uint64) (errno int)

//gocpp:native gp::syscall::sys_lstat
func sysLstat(path string, out []uint64) (errno int)

//gocpp:native gp::syscall::sys_ftruncate
func sysFtruncate(fd int, length int64) (errno int)

//gocpp:native gp::syscall::sys_truncate
func sysTruncate(path string, length int64) (errno int)

//gocpp:native gp::syscall::sys_fsync
func sysFsync(fd int) (errno int)

//gocpp:native gp::syscall::sys_fchmod
func sysFchmod(fd int, mode int) (errno int)

//gocpp:native gp::syscall::sys_chmod
func sysChmod(path string, mode int) (errno int)

//gocpp:native gp::syscall::sys_fchown
func sysFchown(fd int, uid int, gid int) (errno int)

//gocpp:native gp::syscall::sys_chown
func sysChown(path string, uid int, gid int) (errno int)

//gocpp:native gp::syscall::sys_lchown
func sysLchown(path string, uid int, gid int) (errno int)

//gocpp:native gp::syscall::sys_mkdir
func sysMkdir(path string, mode int) (errno int)

//gocpp:native gp::syscall::sys_mknod
func sysMknod(path string, mode int, dev int) (errno int)

//gocpp:native gp::syscall::sys_getrlimit
func sysGetrlimit(resource int) (cur int, max int, errno int)

//gocpp:native gp::syscall::sys_setrlimit
func sysSetrlimit(resource int, cur int, max int) (errno int)

//gocpp:native gp::syscall::sys_rmdir
func sysRmdir(path string) (errno int)

//gocpp:native gp::syscall::sys_unlink
func sysUnlink(path string) (errno int)

//gocpp:native gp::syscall::sys_rename
func sysRename(from string, to string) (errno int)

//gocpp:native gp::syscall::sys_link
func sysLink(oldpath string, newpath string) (errno int)

//gocpp:native gp::syscall::sys_symlink
func sysSymlink(oldpath string, newpath string) (errno int)

//gocpp:native gp::syscall::sys_readlink
func sysReadlink(path string, buf []byte) (n int, errno int)

//gocpp:native gp::syscall::sys_chdir
func sysChdir(path string) (errno int)

//gocpp:native gp::syscall::sys_fchdir
func sysFchdir(fd int) (errno int)

//gocpp:native gp::syscall::sys_getcwd
func sysGetcwd(buf []byte) (n int, errno int)

//gocpp:native gp::syscall::sys_pipe2
func sysPipe2(p []int, flags int) (errno int)

//gocpp:native gp::syscall::sys_getdents
func sysGetdents(fd int, buf []byte) (n int, errno int)

//gocpp:native gp::syscall::sys_utimensat
func sysUtimensat(path string, asec int64, ansec int64, msec int64, mnsec int64) (errno int)

//gocpp:native gp::syscall::sys_dup
func sysDup(fd int) (newfd int, errno int)

//gocpp:native gp::syscall::sys_dup3
func sysDup3(oldfd int, newfd int, flags int) (errno int)

//gocpp:native gp::syscall::sys_umask
func sysUmask(mask int) int

//gocpp:native gp::syscall::sys_set_nonblock
func sysSetNonblock(fd int, nonblocking bool) (errno int)

//gocpp:native gp::syscall::sys_fcntl_getfl
func sysFcntlGetFlForDup(fd int) (flags int, errno int)

//gocpp:native gp::syscall::sys_set_cloexec
func sysSetCloexec(fd int) (errno int)

//gocpp:native gp::syscall::sys_getpid
func sysGetpid() int

//gocpp:native gp::syscall::sys_getppid
func sysGetppid() int

//gocpp:native gp::syscall::sys_getuid
func sysGetuid() int

//gocpp:native gp::syscall::sys_geteuid
func sysGeteuid() int

//gocpp:native gp::syscall::sys_getgid
func sysGetgid() int

//gocpp:native gp::syscall::sys_getegid
func sysGetegid() int

//gocpp:native gp::syscall::sys_getpagesize
func sysGetpagesize() int

//gocpp:native gp::syscall::sys_getgroups
func sysGetgroups(out []int) (n int, errno int)

//gocpp:native gp::syscall::sys_kill
func sysKill(pid int, sig int) (errno int)

//gocpp:native gp::syscall::sys_uname
func sysUname(buf []byte) (errno int)

//gocpp:native gp::syscall::sys_exit
func sysExit(code int)

func Open(path string, mode int, perm uint32) (fd int, err error) {
	if hasNUL(path) {
		return 0, EINVAL // BytePtrFromString fails before the call: the count stays 0
	}
	fd, e := sysOpen(path, mode, int(perm))
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return fd, nil
}

func Close(fd int) (err error) {
	return errnoErr(Errno(sysClose(fd)))
}

func Read(fd int, p []byte) (n int, err error) {
	n, e := sysRead(fd, p)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func Write(fd int, p []byte) (n int, err error) {
	n, e := sysWrite(fd, p)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func Pread(fd int, p []byte, offset int64) (n int, err error) {
	n, e := sysPread(fd, p, offset)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func Pwrite(fd int, p []byte, offset int64) (n int, err error) {
	n, e := sysPwrite(fd, p, offset)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func Seek(fd int, offset int64, whence int) (off int64, err error) {
	off, e := sysSeek(fd, offset, whence)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return off, nil
}

// fillStat sets *st from the numbers of a stat call.
func fillStat(st *Stat_t, b *[16]uint64) {
	st.Dev = b[0]
	st.Ino = b[1]
	st.Nlink = b[2]
	st.Mode = uint32(b[3])
	st.Uid = uint32(b[4])
	st.Gid = uint32(b[5])
	st.Rdev = b[6]
	st.Size = int64(b[7])
	st.Blksize = int64(b[8])
	st.Blocks = int64(b[9])
	st.Atim = Timespec{Sec: int64(b[10]), Nsec: int64(b[11])}
	st.Mtim = Timespec{Sec: int64(b[12]), Nsec: int64(b[13])}
	st.Ctim = Timespec{Sec: int64(b[14]), Nsec: int64(b[15])}
}

func Fstat(fd int, stat *Stat_t) (err error) {
	var b [16]uint64
	if e := sysFstat(fd, b[:]); e != 0 {
		return errnoErr(Errno(e))
	}
	fillStat(stat, &b)
	return nil
}

func Stat(path string, stat *Stat_t) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	var b [16]uint64
	if e := sysStat(path, b[:]); e != 0 {
		return errnoErr(Errno(e))
	}
	fillStat(stat, &b)
	return nil
}

func Lstat(path string, stat *Stat_t) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	var b [16]uint64
	if e := sysLstat(path, b[:]); e != 0 {
		return errnoErr(Errno(e))
	}
	fillStat(stat, &b)
	return nil
}

func Ftruncate(fd int, length int64) (err error) {
	return errnoErr(Errno(sysFtruncate(fd, length)))
}

func Truncate(path string, length int64) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysTruncate(path, length)))
}

func Fsync(fd int) (err error) {
	return errnoErr(Errno(sysFsync(fd)))
}

func Fchmod(fd int, mode uint32) (err error) {
	return errnoErr(Errno(sysFchmod(fd, int(mode))))
}

func Chmod(path string, mode uint32) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysChmod(path, int(mode))))
}

func Fchown(fd int, uid int, gid int) (err error) {
	return errnoErr(Errno(sysFchown(fd, uid, gid)))
}

func Chown(path string, uid int, gid int) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysChown(path, uid, gid)))
}

func Lchown(path string, uid int, gid int) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysLchown(path, uid, gid)))
}

func Mkdir(path string, mode uint32) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysMkdir(path, int(mode))))
}

func Getrlimit(resource int, rlim *Rlimit) (err error) {
	cur, max, errno := sysGetrlimit(resource)
	if errno != 0 {
		return errnoErr(Errno(errno))
	}
	rlim.Cur = uint64(cur)
	rlim.Max = uint64(max)
	return nil
}

func Setrlimit(resource int, rlim *Rlimit) error {
	return errnoErr(Errno(sysSetrlimit(resource, int(rlim.Cur), int(rlim.Max))))
}

func Mknod(path string, mode uint32, dev int) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysMknod(path, int(mode), dev)))
}

func Mkfifo(path string, mode uint32) (err error) {
	return Mknod(path, mode|S_IFIFO, 0)
}

func Rmdir(path string) error {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysRmdir(path)))
}

func Unlink(path string) error {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysUnlink(path)))
}

func Rename(oldpath string, newpath string) (err error) {
	if hasNUL(oldpath) || hasNUL(newpath) {
		return EINVAL
	}
	return errnoErr(Errno(sysRename(oldpath, newpath)))
}

func Link(oldpath string, newpath string) (err error) {
	if hasNUL(oldpath) || hasNUL(newpath) {
		return EINVAL
	}
	return errnoErr(Errno(sysLink(oldpath, newpath)))
}

func Symlink(oldpath string, newpath string) (err error) {
	if hasNUL(oldpath) || hasNUL(newpath) {
		return EINVAL
	}
	return errnoErr(Errno(sysSymlink(oldpath, newpath)))
}

func Readlink(path string, buf []byte) (n int, err error) {
	if hasNUL(path) {
		return 0, EINVAL // BytePtrFromString fails before the call: the count stays 0
	}
	n, e := sysReadlink(path, buf)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func Chdir(path string) (err error) {
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysChdir(path)))
}

func Fchdir(fd int) (err error) {
	return errnoErr(Errno(sysFchdir(fd)))
}

func Getcwd(buf []byte) (n int, err error) {
	n, e := sysGetcwd(buf)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func Getwd() (wd string, err error) {
	var buf [PathMax]byte
	n, err := Getcwd(buf[0:])
	if err != nil {
		return "", err
	}
	// Getcwd returns the number of bytes written to buf, including the NUL.
	if n < 1 || n > len(buf) || buf[n-1] != 0 {
		return "", EINVAL
	}
	// In some cases, Linux can return a path that starts with the
	// "(unreachable)" prefix, which can potentially be a valid relative
	// path. To work around that, return ENOENT if path is not absolute.
	if buf[0] != '/' {
		return "", ENOENT
	}
	return string(buf[0 : n-1]), nil
}

func Pipe(p []int) error {
	return Pipe2(p, 0)
}

func Pipe2(p []int, flags int) error {
	if len(p) != 2 {
		return EINVAL
	}
	var pp [2]int
	if e := sysPipe2(pp[:], flags); e != 0 {
		return errnoErr(Errno(e))
	}
	p[0] = pp[0]
	p[1] = pp[1]
	return nil
}

func Getdents(fd int, buf []byte) (n int, err error) {
	n, e := sysGetdents(fd, buf)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return n, nil
}

func ReadDirent(fd int, buf []byte) (n int, err error) {
	return Getdents(fd, buf)
}

// UtimesNano sets the access and modification times of path to ts[0] and ts[1] (utimensat);
// a Timespec whose Sec and Nsec are both (1<<30)-2 leaves that time as it is.
func UtimesNano(path string, ts []Timespec) (err error) {
	if len(ts) != 2 {
		return EINVAL
	}
	if hasNUL(path) {
		return EINVAL
	}
	return errnoErr(Errno(sysUtimensat(path, ts[0].Sec, ts[0].Nsec, ts[1].Sec, ts[1].Nsec)))
}

func Dup(oldfd int) (fd int, err error) {
	fd, e := sysDup(oldfd)
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	return fd, nil
}

// Dup2 makes newfd a copy of oldfd (dup2(2): newfd is closed first when it is open).
func Dup2(oldfd int, newfd int) (err error) {
	if oldfd == newfd {
		// dup3 fails for equal descriptors, dup2 only checks that oldfd is valid.
		if _, e := sysFcntlGetFlForDup(oldfd); e != 0 {
			return errnoErr(Errno(e))
		}
		return nil
	}
	return errnoErr(Errno(sysDup3(oldfd, newfd, 0)))
}

// Dup3 is Dup2 with flags (O_CLOEXEC).
func Dup3(oldfd int, newfd int, flags int) (err error) {
	return errnoErr(Errno(sysDup3(oldfd, newfd, flags)))
}

// Umask sets the file mode creation mask and returns the old one.
func Umask(mask int) (oldmask int) { return sysUmask(mask) }

// SetNonblock sets or clears O_NONBLOCK on fd.
func SetNonblock(fd int, nonblocking bool) (err error) {
	return errnoErr(Errno(sysSetNonblock(fd, nonblocking)))
}

// CloseOnExec sets the close-on-exec flag of fd.
func CloseOnExec(fd int) { sysSetCloexec(fd) }

func Getpid() (pid int)   { return sysGetpid() }
func Getppid() (ppid int) { return sysGetppid() }
func Getuid() (uid int)   { return sysGetuid() }
func Geteuid() (euid int) { return sysGeteuid() }
func Getgid() (gid int)   { return sysGetgid() }
func Getegid() (egid int) { return sysGetegid() }
func Getpagesize() int    { return sysGetpagesize() }

func Getgroups() (gids []int, err error) {
	n, e := sysGetgroups(nil)
	if e != 0 {
		return nil, errnoErr(Errno(e))
	}
	if n == 0 {
		return nil, nil
	}
	// Sanity check group count. Max is 1<<16 on Linux.
	if n < 0 || n > 1<<20 {
		return nil, EINVAL
	}
	gids = make([]int, n)
	n, e = sysGetgroups(gids)
	if e != 0 {
		return nil, errnoErr(Errno(e))
	}
	return gids[0:n], nil
}

// Kill sends the signal sig to the process pid.
func Kill(pid int, sig Signal) (err error) {
	return errnoErr(Errno(sysKill(pid, int(sig))))
}

// Exit ends the program at once with the code, like os.Exit (it does not run deferred
// functions).
func Exit(code int) { sysExit(code) }

// Uname fills buf with the system's names (uname(2): every field, NUL-padded, as Go's).
func Uname(buf *Utsname) (err error) {
	var b [6 * 65]byte
	if e := sysUname(b[:]); e != 0 {
		return errnoErr(Errno(e))
	}
	for i := 0; i < 65; i++ {
		buf.Sysname[i] = int8(b[i])
		buf.Nodename[i] = int8(b[65+i])
		buf.Release[i] = int8(b[2*65+i])
		buf.Version[i] = int8(b[3*65+i])
		buf.Machine[i] = int8(b[4*65+i])
		buf.Domainname[i] = int8(b[5*65+i])
	}
	return nil
}
