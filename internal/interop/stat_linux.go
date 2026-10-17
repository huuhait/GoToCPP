package interop

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

// words and sysStatFrom write a sysStat as two numbers (the cache on disk)
// and read it back.
func (s sysStat) words() [2]int64 { return [2]int64{int64(s.ino), s.ctime} }

func sysStatFrom(w [2]int64) sysStat { return sysStat{ino: uint64(w[0]), ctime: w[1]} }
