// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package poll

import (
	"errors"
	"sync"
	"syscall"
	"time"
)

// pollDesc is Go's handle on the runtime poller. gocpp has no poller (every goroutine is an OS
// thread), but a descriptor that is a pipe, a FIFO, a terminal or was already in non-blocking mode
// when os got it still has to wait when the kernel says EAGAIN: init accepts it, and a wait is
// ppoll(2) on the descriptor and on a wake descriptor (an eventfd), which blocks the calling thread
// only. Close (evict) and SetDeadline signal the wake descriptor, so a Read or Write that waits is
// ended with ErrFileClosing, or ErrDeadlineExceeded, as in Go. Package os calls init (Init with
// pollable=true) only for such a descriptor; any other one is in blocking mode and its waits are
// never reached (a regular file never waits: no wake descriptor, no extra system call).
//
// The wake descriptor of a mode is made by its first wait that needs it; at most one goroutine
// waits per mode at a time (readLock and writeLock serialize them). When the process has no
// descriptor left for it (EMFILE), the wait is degraded, not failed: it waits without a wake
// descriptor and looks at closing and the deadlines every degradedPoll, so Close and SetDeadline
// still end it (a little later) and a Read or Write that has to wait behaves as in Go.
type pollDesc struct {
	fd int
	ok bool

	mu      sync.Mutex
	closing bool // evict was called: every wait ends with ErrFileClosing
	rd, wd  pollMode
}

// pollMode is the state of one direction.
type pollMode struct {
	deadline int64 // 0: none; < 0: expired; else the nanotime() when it expires
	wake     int   // the wake descriptor (eventfd), when haveWake
	haveWake bool
	waiting  bool // a goroutine is in sysPollWait (so a signal has someone to wake)
}

// degradedPoll is how often (nanoseconds) a wait without a wake descriptor looks at its state.
const degradedPoll = 25e6

var errNoPoller = errors.New("not pollable")

// epoch is the origin of nanotime (the monotonic clock of package time).
var epoch = time.Now()

func nanotime() int64 { return int64(time.Since(epoch)) + 1 }

//gocpp:native gp::syscall::sys_wake_new
func sysWakeNew() (fd int, errno int)

//gocpp:native gp::syscall::sys_wake_signal
func sysWakeSignal(fd int)

//gocpp:native gp::syscall::sys_wake_clear
func sysWakeClear(fd int)

// sysPollWait: state 0 the descriptor is ready, 1 the wake descriptor is readable, 2 the timeout
// (nanoseconds, negative: none) passed.
//
//gocpp:native gp::syscall::sys_poll_wait
func sysPollWait(fd int, wakefd int, write bool, timeoutNs int64) (state int, errno int)

func (pd *pollDesc) init(fd *FD) error {
	pd.fd = fd.Sysfd
	pd.ok = true
	return nil
}

// close releases the wake descriptors. It runs when the last reference is gone, so nobody waits.
func (pd *pollDesc) close() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.ok = false
	if pd.rd.haveWake {
		syscall.Close(pd.rd.wake)
		pd.rd.haveWake = false
	}
	if pd.wd.haveWake {
		syscall.Close(pd.wd.wake)
		pd.wd.haveWake = false
	}
}

// evict unblocks the goroutines that wait on the descriptor: their waits fail with ErrFileClosing,
// and so does every later wait.
func (pd *pollDesc) evict() {
	if !pd.ok {
		return
	}
	pd.mu.Lock()
	pd.closing = true
	pd.rd.signal()
	pd.wd.signal()
	pd.mu.Unlock()
}

func (pd *pollDesc) mode(mode int) *pollMode {
	if mode == 'w' {
		return &pd.wd
	}
	return &pd.rd
}

// signal wakes the goroutine that waits in this mode, if there is one (pd.mu is held). A waiter
// without a wake descriptor sees the change at its next look.
func (m *pollMode) signal() {
	if m.waiting && m.haveWake {
		sysWakeSignal(m.wake)
	}
}

// prepare reports an error for an operation that must not start: the deadline has passed already.
func (pd *pollDesc) prepare(mode int, isFile bool) error {
	if !pd.ok {
		return nil
	}
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if pd.closing {
		return errClosing(isFile)
	}
	if d := pd.mode(mode).deadline; d != 0 && (d < 0 || nanotime() >= d) {
		return ErrDeadlineExceeded
	}
	return nil
}

func (pd *pollDesc) prepareRead(isFile bool) error { return pd.prepare('r', isFile) }

func (pd *pollDesc) prepareWrite(isFile bool) error { return pd.prepare('w', isFile) }

func (pd *pollDesc) wait(mode int, isFile bool) error {
	if !pd.ok {
		return errNoPoller
	}
	m := pd.mode(mode)
	for {
		pd.mu.Lock()
		if pd.closing {
			pd.mu.Unlock()
			return errClosing(isFile)
		}
		timeout := int64(-1)
		if d := m.deadline; d != 0 {
			if d < 0 {
				pd.mu.Unlock()
				return ErrDeadlineExceeded
			}
			timeout = d - nanotime()
			if timeout <= 0 {
				pd.mu.Unlock()
				return ErrDeadlineExceeded
			}
		}
		wake := -1
		if !m.haveWake {
			if fd, e := sysWakeNew(); e == 0 {
				m.wake = fd
				m.haveWake = true
			}
		}
		if m.haveWake {
			wake = m.wake
		} else if timeout < 0 || timeout > degradedPoll {
			timeout = degradedPoll // no wake descriptor (EMFILE): look again soon
		}
		m.waiting = true
		pd.mu.Unlock()

		state, e := sysPollWait(pd.fd, wake, mode == 'w', timeout)

		pd.mu.Lock()
		m.waiting = false
		pd.mu.Unlock()
		if e != 0 {
			return errnoErr(syscall.Errno(e))
		}
		switch state {
		case 0:
			return nil
		case 1:
			// Close or a new deadline: empty the wake descriptor and look at the state again. A
			// Close that signals after this clear has set pd.closing first, so the loop sees it.
			sysWakeClear(wake)
		}
		// state 2 (timeout): the loop finds the deadline passed, or looks again.
	}
}

func (pd *pollDesc) waitRead(isFile bool) error { return pd.wait('r', isFile) }

func (pd *pollDesc) waitWrite(isFile bool) error { return pd.wait('w', isFile) }

func (pd *pollDesc) waitCanceled(mode int) {}

func (pd *pollDesc) pollable() bool { return pd.ok }

// setDeadline sets the deadline of the modes ('r', 'w' or both) d nanoseconds from now (d < 0:
// passed; 0: none) and wakes the goroutines that wait, which look at the new deadline.
func (pd *pollDesc) setDeadline(d int64, mode int) {
	if d > 0 {
		d += nanotime()
		if d <= 0 { // overflow
			d = 1<<63 - 1
		}
	}
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if mode == 'r' || mode == 'r'+'w' {
		pd.rd.deadline = d
		pd.rd.signal()
	}
	if mode == 'w' || mode == 'r'+'w' {
		pd.wd.deadline = d
		pd.wd.signal()
	}
}
