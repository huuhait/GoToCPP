//go:build windows

package main

import (
	"os"

	"github.com/huuhait/gocpp/internal/filelock"
)

// A genLock is an exclusive lock (LockFileEx) on a file: whoever holds it is the one build of its
// program that writes the generated files and compiles them (see lockGen).
type genLock struct{ f *os.File }

// tryLockFile tries to take the lock of the file name without waiting. busy says that another
// holder has it; err is any other failure (a file system without locks).
func tryLockFile(name string) (l *genLock, busy bool, err error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	ok, err := filelock.TryLock(f)
	switch {
	case err != nil:
		f.Close()
		return nil, false, err
	case !ok:
		f.Close()
		return nil, true, nil
	}
	return &genLock{f}, false, nil
}

// unlock releases the lock. (A process that dies releases it too.)
func (l *genLock) unlock() {
	filelock.Unlock(l.f)
	l.f.Close()
}

// lockHolder returns "": this system has no way to read the holder of the lock (Windows locks the
// bytes of the file, so another process can't read it).
func lockHolder(name string) string { return "" }
