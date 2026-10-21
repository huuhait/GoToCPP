//go:build windows

package modfetch

import (
	"errors"
	"os"
	"path/filepath"

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

// openProjectLock opens what LockProject locks. LockFileEx does not lock a folder (it fails with
// "The parameter is incorrect"), so on Windows it is a file of the temporary folder whose name
// comes from the hash of the project's absolute path (lower case): a project is only changed by the gocpp processes of one user, and
// a file inside the project would show up in its listings and in version control.
//
// removeLock deletes the file after it was closed. Windows refuses to delete a file that another
// process has open, so a gocpp that waits for the lock (it has the file open) keeps it, and the
// file never goes away under a holder.
func openProjectLock(dir string) (lock *os.File, removeLock func(), err error) {
	name := filepath.Join(os.TempDir(), "gocpp-project-"+hashName(projectLockKey(dir, true))+".lock")
	lock, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return lock, func() { os.Remove(name) }, nil
}
