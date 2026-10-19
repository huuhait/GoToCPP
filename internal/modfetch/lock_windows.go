//go:build windows

package modfetch

import (
	"errors"
	"os"

	"github.com/huuhait/gocpp/internal/filelock"
)

// flock takes an exclusive lock on the file (LockFileEx), or reports ok=false if another process
// holds the lock (try again). The lock goes away if the process dies.
func flock(f *os.File) (ok bool, err error) {
	ok, err = filelock.TryLock(f)
	if errors.Is(err, filelock.ErrUnsupported) {
		return false, errLockUnsupported
	}
	return ok, err
}

func funlock(f *os.File) { filelock.Unlock(f) }
