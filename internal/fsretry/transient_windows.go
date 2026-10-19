//go:build windows

package fsretry

import (
	"errors"
	"syscall"
)

// transient says that err is one of the errors that pass: ERROR_ACCESS_DENIED (5), ERROR_SHARING_VIOLATION
// (32) and ERROR_LOCK_VIOLATION (33).
func transient(err error) bool {
	var en syscall.Errno
	return errors.As(err, &en) && (en == 5 || en == 32 || en == 33)
}
