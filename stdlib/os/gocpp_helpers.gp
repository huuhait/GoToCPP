// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/bytealg"
	"slices"
	"sync"
	"sync/atomic"
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
	if stathook != nil {
		fi, err := stathook(f, name)
		if fi != nil || err != nil {
			return fi, err
		}
	}
	return Lstat(f.name + "/" + name)
}

// setFmtStdout and setFmtStdin give package fmt the means to use os.Stdout and os.Stdin, the
// variables: fmt.Print, Println and Printf write to the file that os.Stdout holds, and Scan, Scanln and
// Scanf read the file that os.Stdin holds, once the program has assigned another file (or copied one
// over it, *os.Stdout = *w) or closed the original; until then fmt uses descriptors 1 and 0 itself
// (gocpp/fmt.hpp, gocpp/fmtscan.hpp). The program's assignments to os.Stdout, os.Stdin and os.Stderr
// are atomic stores that tell fmt (gp::fmt::store_std, written by the lowering); fmt reads the variable
// through an atomic snapshot and calls write or stdin on that snapshot.
//
//gocpp:native gp::fmt::set_os_stdout
func setFmtStdout(v **File, file0 *file, closed *atomic.Bool, write func(f *File, b []byte) (n int, err error))

//gocpp:native gp::fmt::set_os_stdin
func setFmtStdin(v **File, file0 *file, closed *atomic.Bool, stdin func(f *File) any)

// fmtStdClosing is called by File.Close of the original os.Stdout (true) or os.Stdin (false) after the
// closed flag is set: fmt takes its slow paths. fmtDeferClose is called with the function that closes
// the descriptor, when the descriptor is about to be closed (the close hook, stdclose_*.gp): if a Print
// or Scan is in flight it keeps the function, the last one out closes the descriptor (Go's descriptor
// reference counts do the same), and Close returns at once; so no print reaches a descriptor that the
// next file reuses.
//
//gocpp:native gp::fmt::std_closing
func fmtStdClosing(stdout bool)

//gocpp:native gp::fmt::defer_close
func fmtDeferClose(stdout bool, closeNow func()) bool

// The files the process started with, kept alive, and whether they have been closed (File.Close,
// file_posix.gp, calls noteClose).
var (
	stdout0, stdin0           *file
	stdoutKeep, stdinKeep     *File
	stdoutClosed, stdinClosed atomic.Bool
)

// noteClose is called by Close: it records the close of the original standard streams.
func noteClose(f *File) {
	if f.file == nil {
		return
	}
	if f.file == stdout0 {
		stdoutClosed.Store(true)
		fmtStdClosing(true)
	} else if f.file == stdin0 {
		stdinClosed.Store(true)
		fmtStdClosing(false)
	}
}

func init() {
	stdoutKeep, stdinKeep = Stdout, Stdin
	stdout0, stdin0 = Stdout.file, Stdin.file
	setFmtStdout(&Stdout, stdout0, &stdoutClosed, func(f *File, b []byte) (int, error) { return f.Write(b) })
	setFmtStdin(&Stdin, stdin0, &stdinClosed, func(f *File) any { return f })
	installCloseHook()
}
