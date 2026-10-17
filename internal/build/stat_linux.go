package build

import (
	"io/fs"
	"syscall"
)

// sysStat is the part of a file's stat that only some systems have: on
// Linux, the inode and the change time, which also change when a file is
// replaced by another one with the same size and modification time.
type sysStat struct {
	ino   uint64
	ctime int64 // nanoseconds
}

func sysStatOf(fi fs.FileInfo) sysStat {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return sysStat{ino: st.Ino, ctime: st.Ctim.Nano()}
	}
	return sysStat{}
}
