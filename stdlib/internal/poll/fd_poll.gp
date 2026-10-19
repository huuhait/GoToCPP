// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package poll

import (
	"errors"
	"syscall"
	"time"
)

// pollDesc is Go's handle on the runtime poller. gocpp has no poller (every goroutine is an OS
// thread), but a descriptor that was already in non-blocking mode when os got it (a pipe a parent
// made with O_NONBLOCK, a standard stream shared with such a pipe) still has to wait when the
// kernel says EAGAIN: init accepts it, and a wait is poll(2) on the descriptor, which blocks the
// calling thread only. Package os calls init (Init with pollable=true) only for such a descriptor; any
// other one is in blocking mode and its waits are never reached.
type pollDesc struct {
	fd int
	ok bool
}

var errNoPoller = errors.New("not pollable")

//gocpp:native gp::syscall::sys_poll
func sysPoll(fd int, write bool) (errno int)

func (pd *pollDesc) init(fd *FD) error {
	pd.fd = fd.Sysfd
	pd.ok = true
	return nil
}

func (pd *pollDesc) close() { pd.ok = false }

// evict unblocks the goroutines that wait on the descriptor: a thread that waits in poll(2) is
// not woken (Close does not interrupt a blocked Read).
func (pd *pollDesc) evict() {}

func (pd *pollDesc) prepare(mode int, isFile bool) error { return nil }

func (pd *pollDesc) prepareRead(isFile bool) error { return nil }

func (pd *pollDesc) prepareWrite(isFile bool) error { return nil }

func (pd *pollDesc) wait(mode int, isFile bool) error {
	if !pd.ok {
		return errNoPoller
	}
	if e := sysPoll(pd.fd, mode == 'w'); e != 0 {
		return errnoErr(syscall.Errno(e))
	}
	return nil
}

func (pd *pollDesc) waitRead(isFile bool) error { return pd.wait('r', isFile) }

func (pd *pollDesc) waitWrite(isFile bool) error { return pd.wait('w', isFile) }

func (pd *pollDesc) waitCanceled(mode int) {}

func (pd *pollDesc) pollable() bool { return pd.ok }

// SetDeadline sets the read and write deadlines associated with fd.
func (fd *FD) SetDeadline(t time.Time) error {
	return ErrNoDeadline
}

// SetReadDeadline sets the read deadline associated with fd.
func (fd *FD) SetReadDeadline(t time.Time) error {
	return ErrNoDeadline
}

// SetWriteDeadline sets the write deadline associated with fd.
func (fd *FD) SetWriteDeadline(t time.Time) error {
	return ErrNoDeadline
}

// IsPollDescriptor reports whether fd is the descriptor being used by the poller: never.
func IsPollDescriptor(fd uintptr) bool { return false }
