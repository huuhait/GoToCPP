// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package os

import (
	"internal/bytealg"
	"syscall"
)

// gocpp: the part of Go's root_unix.go (with errSymlink of root_openat.go and isNoFollowErr of
// eloop_other.go) that RemoveAll (removeall_at.gp) needs: os.Root itself is not built
// (stdlib/os/PORT.md). internal/syscall/unix is not ported: Openat, Unlinkat and Readlinkat are
// the natives below, which repeat a call that a signal interrupted themselves.

// sysfdType is the native type of a file handle
// (int on Unix, syscall.Handle on Windows),
// permitting helper functions to be written portably.
type sysfdType = int

// errSymlink reports that a file being operated on is actually a symlink,
// and the target of that symlink.
type errSymlink string

func (errSymlink) Error() string { panic("errSymlink is not user-visible") }

func isNoFollowErr(err error) bool {
	switch err {
	case syscall.ELOOP, syscall.EMLINK:
		return true
	}
	return false
}

func rootOpenDir(parent int, name string) (int, error) {
	var (
		fd  int
		err error
	)
	ignoringEINTR(func() error {
		fd, err = unixOpenat(parent, name, syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_DIRECTORY, 0)
		if isNoFollowErr(err) || err == syscall.ENOTDIR {
			err = checkSymlink(parent, name, err)
		} else if err == syscall.ENOTSUP || err == syscall.EOPNOTSUPP {
			// ENOTSUP and EOPNOTSUPP are often, but not always, the same errno.
			// Translate both to ENOTDIR, since this indicates a non-terminal
			// path component was not a directory.
			err = syscall.ENOTDIR
		}
		return err
	})
	return fd, err
}

func removefileat(fd int, name string) error {
	return ignoringEINTR(func() error {
		return unixUnlinkat(fd, name, 0)
	})
}

func removedirat(fd int, name string) error {
	return ignoringEINTR(func() error {
		return unixUnlinkat(fd, name, atRemovedir)
	})
}

// checkSymlink resolves the symlink name in parent,
// and returns errSymlink with the link contents.
//
// If name is not a symlink, return origError.
func checkSymlink(parent int, name string, origError error) error {
	link, err := readlinkat(parent, name)
	if err != nil {
		return origError
	}
	return errSymlink(link)
}

func readlinkat(fd int, name string) (string, error) {
	for len := 128; ; len *= 2 {
		b := make([]byte, len)
		var (
			n int
			e error
		)
		ignoringEINTR(func() error {
			n, e = unixReadlinkat(fd, name, b)
			return e
		})
		if e == syscall.ERANGE {
			continue
		}
		if e != nil {
			return "", e
		}
		n = max(n, 0)
		if n < len {
			return string(b[0:n]), nil
		}
	}
}

// atRemovedir is unix.AT_REMOVEDIR.
const atRemovedir = 0x200

// unixOpenat, unixUnlinkat and unixReadlinkat are internal/syscall/unix's Openat, Unlinkat and
// Readlinkat (at.go): a name with a NUL byte is EINVAL, as BytePtrFromString makes it, and a
// failure returns the count 0, as Go's do.
func unixOpenat(dirfd int, path string, flags int, perm uint32) (int, error) {
	if bytealg.IndexByteString(path, 0) >= 0 {
		return 0, syscall.EINVAL
	}
	fd, e := sysOpenat(dirfd, path, flags, int(perm))
	if e != 0 {
		return 0, syscall.Errno(e)
	}
	return fd, nil
}

func unixUnlinkat(dirfd int, path string, flags int) error {
	if bytealg.IndexByteString(path, 0) >= 0 {
		return syscall.EINVAL
	}
	if e := sysUnlinkat(dirfd, path, flags); e != 0 {
		return syscall.Errno(e)
	}
	return nil
}

func unixReadlinkat(dirfd int, path string, buf []byte) (int, error) {
	if bytealg.IndexByteString(path, 0) >= 0 {
		return 0, syscall.EINVAL
	}
	n, e := sysReadlinkat(dirfd, path, buf)
	if e != 0 {
		return 0, syscall.Errno(e)
	}
	return n, nil
}

//gocpp:native gp::syscall::sys_openat
func sysOpenat(dirfd int, path string, flags int, perm int) (fd int, errno int)

//gocpp:native gp::syscall::sys_unlinkat
func sysUnlinkat(dirfd int, path string, flags int) (errno int)

//gocpp:native gp::syscall::sys_readlinkat
func sysReadlinkat(dirfd int, path string, buf []byte) (n int, errno int)
