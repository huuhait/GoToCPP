// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// We used to use this code for Darwin, but according to issue #19314
// waitid returns if the process is stopped, even when using WEXITED.

//go:build linux

package os

import (
	"syscall"
)

// blockUntilWaitable attempts to block until a call to p.Wait will
// succeed immediately, and reports whether it has done so.
// It does not actually call p.Wait.
//
// gocpp: Go's wait_waitid.go; unix.Waitid is the native sysWaitid, which repeats a call that a
// signal interrupted (Go's ignoringEINTR) and drops the siginfo (Go's is unused too).
func (p *Process) blockUntilWaitable() (bool, error) {
	e := sysWaitid(p.Pid, syscall.WEXITED|syscall.WNOWAIT)
	if e != 0 {
		err := syscall.Errno(e)
		// waitid has been available since Linux 2.6.9, but
		// reportedly is not available in Ubuntu on Windows.
		// See issue 16610.
		if err == syscall.ENOSYS {
			return false, nil
		}
		return false, NewSyscallError("waitid", err)
	}
	return true, nil
}

// sysWaitid is waitid(2) with P_PID: it returns the errno (0 for success).
//
//gocpp:native gp::syscall::sys_waitid
func sysWaitid(pid int, options int) (errno int)
