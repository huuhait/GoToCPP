//go:build unix

package load

import (
	"errors"
	"syscall"
)

// openNoFollow makes os.OpenFile refuse a symbolic link and not wait for a named pipe.
const openNoFollow = syscall.O_NOFOLLOW | syscall.O_NONBLOCK

// openNonblock makes os.OpenFile not wait for a writer when the name is a named pipe (the open of
// a file or folder is not changed by it).
const openNonblock = syscall.O_NONBLOCK

// isNoDevice reports whether err is ENXIO, what the open of a socket gives.
func isNoDevice(err error) bool { return errors.Is(err, syscall.ENXIO) }
