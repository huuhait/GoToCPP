// Code ported from Go 1.25.14 src/io/fs/readdir.go; DO NOT EDIT by hand; see PORT.md.

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"errors"
)

// ReadDirFS is the interface implemented by a file system
// that provides an optimized implementation of [ReadDir].
type ReadDirFS interface {
	FS

	// ReadDir reads the named directory
	// and returns a list of directory entries sorted by filename.
	ReadDir(name string) ([]DirEntry, error)
}

// ReadDir reads the named directory
// and returns a list of directory entries sorted by filename.
//
// If fs implements [ReadDirFS], ReadDir calls fs.ReadDir.
// Otherwise ReadDir calls fs.Open and uses ReadDir and Close
// on the returned file.
func ReadDir(fsys FS, name string) ([]DirEntry, error) {
	if fsys, ok := fsys.(ReadDirFS); ok {
		return fsys.ReadDir(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir, ok := file.(ReadDirFile)
	if !ok {
		return nil, &PathError{Op: "readdir", Path: name, Err: errors.New("not implemented")}
	}

	list, err := dir.ReadDir(-1)
	sortDirEntries(list)
	return list, err
}

// sortDirEntries sorts list by name (the byte order of the names). Go calls slices.SortFunc with
// bytealg.CompareString; package slices is not real yet (stdlib/io/fs/PORT.md).
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

// dirInfo is a DirEntry based on a FileInfo.
type dirInfo struct {
	fileInfo FileInfo
}

func (di dirInfo) IsDir() bool {
	return di.fileInfo.IsDir()
}

func (di dirInfo) Type() FileMode {
	return di.fileInfo.Mode().Type()
}

func (di dirInfo) Info() (FileInfo, error) {
	return di.fileInfo, nil
}

func (di dirInfo) Name() string {
	return di.fileInfo.Name()
}

func (di dirInfo) String() string {
	return FormatDirEntry(di)
}

// FileInfoToDirEntry returns a [DirEntry] that returns information from info.
// If info is nil, FileInfoToDirEntry returns nil.
func FileInfoToDirEntry(info FileInfo) DirEntry {
	if info == nil {
		return nil
	}
	return dirInfo{fileInfo: info}
}
