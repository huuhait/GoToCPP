//go:build linux

package probe

import (
	"os"
	"syscall"
)

// changeTime returns the inode change time of a file in nanoseconds, which the system sets at
// every write and every change of the file's times and which a program cannot set back: a
// check by size and modification time that includes it cannot be fooled by an edit that
// restores the modification time. ok is false where the system does not have it.
func changeTime(fi os.FileInfo) (ns int64, ok bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int64(st.Ctim.Sec)*1e9 + int64(st.Ctim.Nsec), true
}
