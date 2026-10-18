//go:build unix

package main

import (
	"os"
	"syscall"
)

// A genLock is an advisory lock (flock) on a file: whoever holds it is the one build of its program
// that writes the generated files and compiles them (see lockGen).
type genLock struct{ f *os.File }

// tryLockFile tries to take the lock of the file name without waiting. busy says that another
// holder has it; err is any other failure (a file system without locks).
func tryLockFile(name string) (l *genLock, busy bool, err error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	switch err {
	case nil:
		return &genLock{f}, false, nil
	case syscall.EWOULDBLOCK:
		f.Close()
		return nil, true, nil
	}
	f.Close()
	return nil, false, err
}

// unlock releases the lock. (A process that dies releases it too.)
func (l *genLock) unlock() {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}
