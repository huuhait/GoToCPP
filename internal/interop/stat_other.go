//go:build !linux

package interop

import "io/fs"

// sysStat is the part of a file's stat that only some systems have (see
// stat_linux.go); elsewhere the size and the modification time are used
// alone.
type sysStat struct{}

func sysStatOf(fs.FileInfo) sysStat { return sysStat{} }

// words and sysStatFrom write a sysStat as two numbers (the cache on disk)
// and read it back.
func (sysStat) words() [2]int64 { return [2]int64{} }

func sysStatFrom([2]int64) sysStat { return sysStat{} }
