//go:build !unix

package build

// A fileLock is not available on this system: builds that run at the same time may make the same
// runtime library twice (each renames its own into place, so none is ever half written).
type fileLock struct{}

func tryLockFile(name string) (l *fileLock, busy bool, err error) { return nil, false, nil }

func (l *fileLock) unlock() {}
