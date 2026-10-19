// Package filelock takes advisory, exclusive locks on open files, without waiting: flock on Unix and
// LockFileEx on Windows (Phase 14 step 14.2). A lock lives as long as the file is open, and the
// system releases it when the process ends, so a crashed gocpp never leaves one behind. The lock
// is between processes (and between separate opens of the file in one process); it is how two
// gocpp builds that start at once share one toolchain download, one runtime library or one
// generated folder.
package filelock

import "errors"

// ErrUnsupported is the error of a system or a file system that has no locks (the caller goes on
// without the lock: the cost is duplicate work, never a wrong result).
var ErrUnsupported = errors.New("file locks are not supported here")
