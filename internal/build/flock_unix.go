//go:build unix

package build

import (
	"os"
	"syscall"
)

// A fileLock is an advisory lock (flock) on a file: whoever holds it is the one build of the file's
// subject (the runtime library of a key, rtlib.go). A process that dies releases it.
type fileLock struct{ f *os.File }

// tryLockFile tries to take the lock of the file name without waiting. busy says that another
// holder has it; err is any other failure (a file system without locks): the caller goes on
// without the lock, which only costs duplicate work (the results are renamed into place).
func tryLockFile(name string) (l *fileLock, busy bool, err error) {
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
		return &fileLock{f}, false, nil
	case syscall.EWOULDBLOCK:
		f.Close()
		return nil, true, nil
	}
	f.Close()
	return nil, false, err
}

func (l *fileLock) unlock() {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}
