//go:build !unix

package modfetch

import "os"

// There is no file lock here: the downloads of two gocpp processes at once
// stay correct (each extracts into a folder of its own and the rename into
// the cache has one winner) but both download.
func flock(f *os.File) (bool, error) { return true, nil }

func funlock(f *os.File) {}
