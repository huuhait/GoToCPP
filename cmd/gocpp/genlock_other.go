//go:build !unix && !windows

package main

import "errors"

// A genLock is a lock on a file; this system has none (Unix has flock, Windows LockFileEx),
// so lockGen gives each build a private folder.
type genLock struct{}

func tryLockFile(name string) (*genLock, bool, error) {
	return nil, false, errors.New("file locks are not supported on this system")
}

func (l *genLock) unlock() {}
