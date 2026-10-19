//go:build !unix && !windows

package filelock

import "os"

// TryLock has no lock here.
func TryLock(*os.File) (bool, error) { return false, ErrUnsupported }

// Unlock does nothing here.
func Unlock(*os.File) {}
