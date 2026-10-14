// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gocpp: the parser of Go's internal/txtar package (archive.go, Go
// 1.25.14), which diff_test.go uses to read its test files. A package
// outside Go's standard library can't import internal/txtar. The changes:
// the code is in package diff, not package txtar; the names say what they
// are in package diff (Archive is txtarArchive, File is txtarFile, Parse
// is parseTxtar, ParseFile is parseTxtarFile); and the package doc and
// Format are left out, with the import of fmt, which only Format needs.
// The rest of the code is the same.

package diff

import (
	"bytes"
	"os"
	"strings"
)

// A txtarArchive is a collection of files.
type txtarArchive struct {
	Comment []byte
	Files   []txtarFile
}

// A txtarFile is a single file in an archive.
type txtarFile struct {
	Name string // name of file ("foo/bar.txt")
	Data []byte // text content of file
}

// parseTxtarFile parses the named file as an archive.
func parseTxtarFile(file string) (*txtarArchive, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parseTxtar(data), nil
}

// parseTxtar parses the serialized form of an Archive.
// The returned Archive holds slices of data.
func parseTxtar(data []byte) *txtarArchive {
	a := new(txtarArchive)
	var name string
	a.Comment, name, data = findFileMarker(data)
	for name != "" {
		f := txtarFile{name, nil}
		f.Data, name, data = findFileMarker(data)
		a.Files = append(a.Files, f)
	}
	return a
}

var (
	newlineMarker = []byte("\n-- ")
	marker        = []byte("-- ")
	markerEnd     = []byte(" --")
)

// findFileMarker finds the next file marker in data,
// extracts the file name, and returns the data before the marker,
// the file name, and the data after the marker.
// If there is no next marker, findFileMarker returns before = fixNL(data), name = "", after = nil.
func findFileMarker(data []byte) (before []byte, name string, after []byte) {
	var i int
	for {
		if name, after = isMarker(data[i:]); name != "" {
			return data[:i], name, after
		}
		j := bytes.Index(data[i:], newlineMarker)
		if j < 0 {
			return fixNL(data), "", nil
		}
		i += j + 1 // positioned at start of new possible marker
	}
}

// isMarker checks whether data begins with a file marker line.
// If so, it returns the name from the line and the data after the line.
// Otherwise it returns name == "" with an unspecified after.
func isMarker(data []byte) (name string, after []byte) {
	if !bytes.HasPrefix(data, marker) {
		return "", nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		data, after = data[:i], data[i+1:]
	}
	if !(bytes.HasSuffix(data, markerEnd) && len(data) >= len(marker)+len(markerEnd)) {
		return "", nil
	}
	return strings.TrimSpace(string(data[len(marker) : len(data)-len(markerEnd)])), after
}

// If data is empty or ends in \n, fixNL returns data.
// Otherwise fixNL returns a new slice consisting of data with a final \n added.
func fixNL(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	d := make([]byte, len(data)+1)
	copy(d, data)
	d[len(data)] = '\n'
	return d
}
