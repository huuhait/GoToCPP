//go:build unix

package server

import (
	"os"
	"syscall"
)

// ownedByUs reports whether the file belongs to the user of the process.
func ownedByUs(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// pidAlive reports whether a process with the id pid exists.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
