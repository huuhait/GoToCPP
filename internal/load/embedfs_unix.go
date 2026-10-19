//go:build unix

package load

import "syscall"

// openNoFollow makes os.OpenFile refuse a symbolic link and not wait for a named pipe.
const openNoFollow = syscall.O_NOFOLLOW | syscall.O_NONBLOCK
