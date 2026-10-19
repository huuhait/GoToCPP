//go:build windows

package filelock

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2

	errorLockViolation    = syscall.Errno(33) // ERROR_LOCK_VIOLATION: a byte range that another holder has
	errorIOPending        = syscall.Errno(997)
	errorSharingViolation = syscall.Errno(32)
	errorNotSupported     = syscall.Errno(50)
)

// lockRange is the range that is locked: the first byte, like Go's own cmd/go/internal/lockedfile. A
// lock on a byte range of a file is mandatory on Windows for the file's other handles that read or
// write that byte, so the byte is far out, where no program reads (and the files that are locked
// are empty marker files).
const lockOffsetHigh = 0x7fffffff

// TryLock takes the exclusive lock of f without waiting (LockFileEx with LOCKFILE_FAIL_IMMEDIATELY).
// ok is false when another holder has it.
func TryLock(f *os.File) (ok bool, err error) {
	var ol syscall.Overlapped
	ol.OffsetHigh = lockOffsetHigh
	r, _, e := procLockFileEx.Call(f.Fd(), lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r != 0 {
		return true, nil
	}
	switch e {
	case errorLockViolation, errorIOPending, errorSharingViolation:
		return false, nil
	case errorNotSupported:
		return false, ErrUnsupported
	}
	return false, e
}

// Unlock releases the lock of f (closing the file does too).
func Unlock(f *os.File) {
	var ol syscall.Overlapped
	ol.OffsetHigh = lockOffsetHigh
	procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
}
