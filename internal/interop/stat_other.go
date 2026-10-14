//go:build !linux

package interop

import "io/fs"

// sysStat is the part of a file's stat that only some systems have (see
// stat_linux.go); elsewhere the size and the modification time are used
// alone.
type sysStat struct{}

func sysStatOf(fs.FileInfo) sysStat { return sysStat{} }
