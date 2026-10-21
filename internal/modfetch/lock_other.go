//go:build !unix && !windows

package modfetch

import "os"

// There is no file lock here: the downloads of two gocpp processes at once
// stay correct (each extracts into a folder of its own and the rename into
// the cache has one winner) but both download.
func flock(f *os.File) (bool, error) { return true, nil }

func funlock(f *os.File) {}

// openProjectLock opens the folder of the project, which is what LockProject "locks" (there is no
// file lock here). removeLock has nothing to do.
func openProjectLock(dir string) (lock *os.File, removeLock func(), err error) {
	lock, err = os.Open(dir)
	return lock, func() {}, err
}
