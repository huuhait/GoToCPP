//go:build unix

package tests

import (
	"io/fs"
	"os"
	"syscall"
)

// ownedByMe says whether the file belongs to this user.
func ownedByMe(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
