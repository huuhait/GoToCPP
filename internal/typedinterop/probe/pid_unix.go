//go:build unix

package probe

import (
	"errors"
	"syscall"
)

// pidAlive reports whether a process with the id exists (a process of another user does).
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
