//go:build !unix

package probe

import "os"

// pidAlive reports whether a process with the id exists: os.FindProcess opens it on Windows and
// fails for one that is gone. Elsewhere it is assumed to be (the time limit decides).
func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	p.Release()
	return true
}
