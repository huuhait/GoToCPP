//go:build darwin

package probe

import (
	"os"
	"syscall"
)

// changeTime: see statid_linux.go.
func changeTime(fi os.FileInfo) (ns int64, ok bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int64(st.Ctimespec.Sec)*1e9 + int64(st.Ctimespec.Nsec), true
}
