//go:build !unix

package main

import "errors"

// A genLock is a lock on a file; this system has none yet (Windows: LockFileEx comes with Phase 14),
// so lockGen gives each build a private folder.
type genLock struct{}

func tryLockFile(name string) (*genLock, bool, error) {
	return nil, false, errors.New("file locks are not supported on this system")
}

func (l *genLock) unlock() {}
