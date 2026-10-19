//go:build unix

package toolchain

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process with this number exists (it may belong to another user: then it exists).
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
