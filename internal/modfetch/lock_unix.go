//go:build unix

package modfetch

import (
	"os"
	"syscall"
)

// flock takes an exclusive lock on the file and returns the function that
// releases it, or ok=false if another process holds the lock (try again).
// The lock goes away if the process dies.
func flock(f *os.File) (ok bool, err error) {
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
			return false, errLockUnsupported
		}
		return false, err
	}
}

func funlock(f *os.File) { syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

// openProjectLock opens what LockProject locks: the folder of the project itself, which leaves no
// file in it. removeLock has nothing to do here.
func openProjectLock(dir string) (lock *os.File, removeLock func(), err error) {
	lock, err = os.Open(dir)
	return lock, func() {}, err
}
