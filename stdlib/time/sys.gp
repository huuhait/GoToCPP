// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import "errors"

// The files of the time zone database are read through native functions (runtime/src/time.cpp, C
// stdio, so the same code serves every OS): Go's sys_unix.go calls package syscall, which gocpp does
// not have. A native function returns the errno of a failure, 0 for success.

// errENOENT is Go's syscall.ENOENT: the file does not exist (the time zone loaders go on to the
// next source of zone data when they get it).
var errENOENT error = errors.New("no such file or directory")

// errnoError returns the error of an errno, nil for 0.
func errnoError(errno int) error {
	switch errno {
	case 0:
		return nil
	case 2: // ENOENT
		return errENOENT
	}
	return errors.New(errnoText(errno))
}

// errnoText is the text of an errno (strerror, with the first letter in lower case, as Go's
// syscall.Errno prints it).
//
//gocpp:native gp::time::errno_text
func errnoText(errno int) string

//gocpp:native gp::time::open_file
func openFile(name string) (fd int, errno int)

//gocpp:native gp::time::read_file
func readFd(fd int, buf []byte) (n int, errno int)

//gocpp:native gp::time::close_file
func closeFd(fd int)

//gocpp:native gp::time::seek_file
func seekFd(fd int, off int64, whence int) (errno int)

func open(name string) (uintptr, error) {
	fd, errno := openFile(name)
	if errno != 0 {
		return 0, errnoError(errno)
	}
	return uintptr(fd), nil
}

func read(fd uintptr, buf []byte) (int, error) {
	n, errno := readFd(int(fd), buf)
	return n, errnoError(errno)
}

func closefd(fd uintptr) {
	closeFd(int(fd))
}

func preadn(fd uintptr, buf []byte, off int) error {
	whence := seekStart
	if off < 0 {
		whence = seekEnd
	}
	if errno := seekFd(int(fd), int64(off), whence); errno != 0 {
		return errnoError(errno)
	}
	for len(buf) > 0 {
		m, errno := readFd(int(fd), buf)
		if m <= 0 {
			if errno == 0 {
				return errors.New("short read")
			}
			return errnoError(errno)
		}
		buf = buf[m:]
	}
	return nil
}

// interrupt is for testing: whatever interrupts a sleep. Go sends SIGCHLD to the process. A gocpp
// sleep is a wait of the scheduler that no signal ends, so there is nothing to interrupt.
func interrupt() {}
