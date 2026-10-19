//go:build unix

package filelock

import (
	"os"
	"syscall"
)

// TryLock takes the exclusive lock of f without waiting. ok is false when another holder has it; err
// is any other failure (ErrUnsupported on a file system without locks).
func TryLock(f *os.File) (ok bool, err error) {
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch err {
		case nil:
			return true, nil
		case syscall.EINTR:
			continue
		case syscall.EWOULDBLOCK:
			return false, nil
		case syscall.ENOLCK, syscall.EOPNOTSUPP, syscall.ENOSYS:
			return false, ErrUnsupported
		}
		return false, err
	}
}

// Unlock releases the lock of f (closing the file does too).
func Unlock(f *os.File) { syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
