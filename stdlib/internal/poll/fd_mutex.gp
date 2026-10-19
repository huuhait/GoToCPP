// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package poll

import (
	"sync"
	"sync/atomic"
)

// fdMutex is a specialized synchronization primitive that manages
// lifetime of an fd and serializes access to Read, Write and Close
// methods on FD.
//
// Go's version keeps the two locks in its state word and parks the waiters on runtime
// semaphores. This one keeps the closed bit and the reference count in one atomic word
// and the two locks in sync.Mutex values; the rules are the same: a reference keeps the
// descriptor open, Close sets the closed bit and drops its own reference, and the last
// reference to go closes the descriptor. A caller that waits for a lock when Close
// happens gets "closed" once it has the lock, like Go's woken waiters.
type fdMutex struct {
	state atomic.Uint64
	rmu   sync.Mutex
	wmu   sync.Mutex
}

const (
	mutexClosed  = 1 << 0
	mutexRef     = 1 << 1
	mutexRefMask = (1<<40 - 1) << 1
)

const overflowMsg = "too many concurrent operations on a single file or socket (max 1048575)"

// incref adds a reference to mu.
// It reports whether mu is available for reading or writing.
func (mu *fdMutex) incref() bool {
	for {
		old := mu.state.Load()
		if old&mutexClosed != 0 {
			return false
		}
		new := old + mutexRef
		if new&mutexRefMask == 0 {
			panic(overflowMsg)
		}
		if mu.state.CompareAndSwap(old, new) {
			return true
		}
	}
}

// increfAndClose sets the state of mu to closed.
// It adds a reference but returns false if the mutex is already closed.
func (mu *fdMutex) increfAndClose() bool {
	for {
		old := mu.state.Load()
		if old&mutexClosed != 0 {
			return false
		}
		// Mark as closed and acquire a reference.
		new := (old | mutexClosed) + mutexRef
		if new&mutexRefMask == 0 {
			panic(overflowMsg)
		}
		if mu.state.CompareAndSwap(old, new) {
			return true
		}
	}
}

// decref removes a reference from mu.
// It reports whether there is no remaining reference.
func (mu *fdMutex) decref() bool {
	for {
		old := mu.state.Load()
		if old&mutexRefMask == 0 {
			panic("inconsistent poll.fdMutex")
		}
		new := old - mutexRef
		if mu.state.CompareAndSwap(old, new) {
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

// rwlock adds a reference to mu and locks mu for reading or writing.
// It reports whether mu is available for reading or writing, and, when it is not because Close
// happened while the call waited for the lock, whether the call dropped the last reference (the
// caller then destroys the descriptor).
func (mu *fdMutex) rwlock(read bool) (ok, last bool) {
	if !mu.incref() {
		return false, false
	}
	if read {
		mu.rmu.Lock()
	} else {
		mu.wmu.Lock()
	}
	if mu.state.Load()&mutexClosed != 0 {
		// Close happened while this call waited for the lock.
		if read {
			mu.rmu.Unlock()
		} else {
			mu.wmu.Unlock()
		}
		return false, mu.decref()
	}
	return true, false
}

// rwunlock unlocks mu for reading or writing and removes a reference.
// It reports whether there is no remaining reference.
func (mu *fdMutex) rwunlock(read bool) bool {
	if read {
		mu.rmu.Unlock()
	} else {
		mu.wmu.Unlock()
	}
	return mu.decref()
}

// Read operations must do rwlock(true) and rwunlock(true).
//
// Write operations must do rwlock(false) and rwunlock(false).
//
// Misc operations must do incref/decref.
// Misc operations include functions like setsockopt and setDeadline.
// They need to use incref/decref to ensure that they operate on the
// correct fd in presence of a concurrent close call (otherwise fd can
// be closed under their feet).
//
// Close operation must do increfAndClose/decref.

// readLock adds a reference to fd and locks fd for reading.
// It returns an error when fd cannot be used for reading.
func (fd *FD) readLock() error {
	ok, last := fd.fdmu.rwlock(true)
	if last {
		fd.destroy()
	}
	if !ok {
		return errClosing(fd.isFile)
	}
	return nil
}

// readUnlock removes a reference from fd and unlocks fd for reading.
// It destroys fd if there are no remaining references.
func (fd *FD) readUnlock() {
	if fd.fdmu.rwunlock(true) {
		fd.destroy()
	}
}

// writeLock adds a reference to fd and locks fd for writing.
// It returns an error when fd cannot be used for writing.
func (fd *FD) writeLock() error {
	ok, last := fd.fdmu.rwlock(false)
	if last {
		fd.destroy()
	}
	if !ok {
		return errClosing(fd.isFile)
	}
	return nil
}

// writeUnlock removes a reference from fd and unlocks fd for writing.
// It destroys fd if there are no remaining references.
func (fd *FD) writeUnlock() {
	if fd.fdmu.rwunlock(false) {
		fd.destroy()
	}
}

// incref adds a reference to fd.
// It returns an error when fd cannot be used.
func (fd *FD) incref() error {
	if !fd.fdmu.incref() {
		return errClosing(fd.isFile)
	}
	return nil
}

// decref removes a reference from fd.
// It also closes fd when the state of fd is closing and there
// is no remaining reference.
func (fd *FD) decref() error {
	if fd.fdmu.decref() {
		return fd.destroy()
	}
	return nil
}
