//go:build windows

package build

import (
	"os"

	"github.com/huuhait/gocpp/internal/filelock"
)

// A fileLock is an exclusive lock (LockFileEx) on a file: whoever holds it is the one build of the
// file's subject (the runtime library of a key, rtlib.go). A process that dies releases it.
type fileLock struct{ f *os.File }

// tryLockFile tries to take the lock of the file name without waiting. busy says that another
// holder has it; err is any other failure (a file system without locks): the caller goes on
// without the lock, which only costs duplicate work (the results are renamed into place).
func tryLockFile(name string) (l *fileLock, busy bool, err error) {
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
	return &fileLock{f}, false, nil
}

func (l *fileLock) unlock() {
	filelock.Unlock(l.f)
	l.f.Close()
}
