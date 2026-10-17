//go:build !linux

package build

import "io/fs"

// sysStat is the part of a file's stat that only some systems have (see
// stat_linux.go); elsewhere it is always zero, and the size and the
// modification time are used alone (the racy window protects them).
type sysStat struct {
	ino   uint64
	ctime int64
}

func sysStatOf(fs.FileInfo) sysStat { return sysStat{} }
