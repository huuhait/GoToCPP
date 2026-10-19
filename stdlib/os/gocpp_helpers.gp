// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"io"
	"sync"
)

// Helpers that stand in for generic functions of the standard library that gocpp's ports do not
// have yet (slices.SortFunc, slices.Grow): stdlib/os/PORT.md.

// sortDirEntries sorts list by name (the byte order of the names), as os.ReadDir does with
// slices.SortFunc and bytealg.CompareString.
func sortDirEntries(list []DirEntry) {
	if len(list) < 2 {
		return
	}
	tmp := make([]DirEntry, len(list))
	mergeSortEntries(list, tmp)
}

func mergeSortEntries(a, tmp []DirEntry) {
	if len(a) <= 12 {
		for i := 1; i < len(a); i++ {
			for j := i; j > 0 && a[j].Name() < a[j-1].Name(); j-- {
				a[j], a[j-1] = a[j-1], a[j]
			}
		}
		return
	}
	mid := len(a) / 2
	mergeSortEntries(a[:mid], tmp[:mid])
	mergeSortEntries(a[mid:], tmp[mid:])
	copy(tmp, a)
	i, j, k := 0, mid, 0
	for i < mid && j < len(a) {
		if tmp[j].Name() < tmp[i].Name() {
			a[k] = tmp[j]
			j++
		} else {
			a[k] = tmp[i]
			i++
		}
		k++
	}
	for i < mid {
		a[k] = tmp[i]
		i++
		k++
	}
	for j < len(a) {
		a[k] = tmp[j]
		j++
		k++
	}
}

// growBytes is slices.Grow for a []byte: it returns s with room for n more bytes.
func growBytes(s []byte, n int) []byte {
	if n -= cap(s) - len(s); n > 0 {
		t := make([]byte, len(s), cap(s)+n)
		copy(t, s)
		s = t
	}
	return s
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
