// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/bytealg"
	"io"
	"slices"
	"sync"
)

// Helpers for the generic functions of package slices (which the 9.6 version of this package could not call, so it had its own merge
// sort and Grow): stdlib/os/PORT.md.

// sortDirEntries sorts list by name (the byte order of the names), as os.ReadDir does.
func sortDirEntries(list []DirEntry) {
	slices.SortFunc(list, func(a, b DirEntry) int {
		return bytealg.CompareString(a.Name(), b.Name())
	})
}

// growBytes is slices.Grow for a []byte: it returns s with room for n more bytes.
func growBytes(s []byte, n int) []byte {
	return slices.Grow(s, n)
}

// dirInfoPointer is atomic.Pointer[dirInfo] (Go's field type), which gocpp's sync/atomic does not
// have until its generic types are built: a pointer behind a mutex, with the three methods os uses.
type dirInfoPointer struct {
	mu sync.Mutex
	p  *dirInfo
}

func (d *dirInfoPointer) Load() *dirInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.p
}

func (d *dirInfoPointer) Swap(new *dirInfo) *dirInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	old := d.p
	d.p = new
	return old
}

func (d *dirInfoPointer) CompareAndSwap(old, new *dirInfo) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.p != old {
		return false
	}
	d.p = new
	return true
}

// supportsCreateWithStickyBit reports whether the system's open(2) honours the sticky bit when it
// creates a file (Linux does; Go's sticky_notbsd.go).
const supportsCreateWithStickyBit = true

// lstatat is the Lstat of the entry name of the directory f (Go's version uses fstatat on the open
// directory, for a directory opened in a Root; there is no Root here).
func (f *File) lstatat(name string) (FileInfo, error) {
	return Lstat(f.name + "/" + name)
}

// readFrom and writeTo are the zero-copy paths of ReadFrom and WriteTo (copy_file_range, sendfile,
// splice on Linux): gocpp has none, so the generic io.Copy is used.
func (f *File) readFrom(r io.Reader) (n int64, handled bool, err error) {
	return 0, false, nil
}

func (f *File) writeTo(w io.Writer) (n int64, handled bool, err error) {
	return 0, false, nil
}
