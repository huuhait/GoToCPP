// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gocpp: readFile, writeFile and backupFile of cmd/gofmt/gofmt.go (Go
// 1.25.14), so that gocpp fmt (cmd/gocpp) reads and writes files the way
// gofmt does. The changes:
//   - The three functions are in package format, not package main, and
//     readFile is ReadFile and writeFile is WriteFile. ReadFile reads only
//     files: gocpp fmt reads standard input itself, so the in parameter is
//     gone.
//   - There is no limit on open files (fdSem): gocpp fmt formats one file
//     at a time.
//   - When WriteFile can't restore the file after a failed write, gofmt
//     prints a message to its standard error and returns the error of the
//     write. A package shouldn't print, so WriteFile returns both, joined
//     with errors.Join (restoreFail): the message, then the error, which
//     is the order in which gofmt prints them. The message has no "gofmt: "
//     at its start.
//   - When backupFile can't write the whole backup (the disk is full, or
//     the file is too big), it removes the part that it wrote. gofmt leaves
//     it.

package format

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/huuhait/gocpp/internal/load"
)

// ReadFile reads the file filename, which info describes, like gofmt reads
// a file. If info says that it is a regular file, ReadFile returns an
// error if the size of the file changes while it reads it: someone may be
// editing the file at the same time, and writing back a formatted half
// file would destroy their work.
func ReadFile(filename string, info fs.FileInfo) ([]byte, error) {
	// (a named pipe or a device is an error, not a wait for a writer)
	in, err := load.OpenRegular(filename)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	// Compute the file's size and read its contents with minimal allocations.
	//
	// If we have the FileInfo from filepath.WalkDir, use it to make
	// a buffer of the right size and avoid ReadAll's reallocations.
	//
	// If the size is unknown (or bogus, or overflows an int), fall back to
	// a size-independent ReadAll.
	size := -1
	if info != nil && info.Mode().IsRegular() && int64(int(info.Size())) == info.Size() {
		size = int(info.Size())
	}
	if size+1 <= 0 {
		// The file is not known to be regular, so we don't have a reliable size for it.
		var err error
		src, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		return src, nil
	}

	// We try to read size+1 bytes so that we can detect modifications: if we
	// read more than size bytes, then the file was modified concurrently.
	// (If that happens, we could, say, append to src to finish the read, or
	// proceed with a truncated buffer — but the fact that it changed at all
	// indicates a possible race with someone editing the file, so we prefer to
	// stop to avoid corrupting it.)
	src := make([]byte, size+1)
	n, err := io.ReadFull(in, src)
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		// io.ReadFull returns io.EOF (for an empty file) or io.ErrUnexpectedEOF
		// (for a non-empty file) if the file was changed unexpectedly. Continue
		// with comparing file sizes in those cases.
	default:
		return nil, err
	}
	if n < size {
		return nil, fmt.Errorf("error: size of %s changed during reading (from %d to %d bytes)", filename, size, n)
	} else if n > size {
		return nil, fmt.Errorf("error: size of %s changed during reading (from %d to >=%d bytes)", filename, size, len(src))
	}
	return src[:n], nil
}

// WriteFile replaces the contents of the file filename with formatted, the
// way gofmt -w does. orig is what the file holds now, size is its size and
// perm its permissions. WriteFile first copies orig to a new backup file
// next to it, named filename.<random number>, with the permissions perm.
// Then it writes formatted into the file itself, so the file keeps its
// mode, its owner and its links. If that write fails, it puts orig back.
// At the end it removes the backup. Only when something goes wrong after
// the file has changed, and WriteFile can't be sure that the file is
// right, it keeps the backup, and the error says where it is.
func WriteFile(filename string, orig, formatted []byte, perm fs.FileMode, size int64) error {
	// Make a temporary backup file before rewriting the original file.
	bakname, err := backupFile(filename, orig, perm)
	if err != nil {
		return err
	}

	fout, err := os.OpenFile(filename, os.O_WRONLY, perm)
	if err != nil {
		// We couldn't even open the file, so it should
		// not have changed.
		os.Remove(bakname)
		return err
	}
	defer fout.Close() // for error paths

	// gocpp: gofmt prints this message and then returns err.
	restoreFail := func(err, erro error) error {
		return errors.Join(fmt.Errorf("%s: error restoring file to original: %v; backup in %s", filename, erro, bakname), err)
	}

	n, err := fout.Write(formatted)
	if err == nil && int64(n) < size {
		err = fout.Truncate(int64(n))
	}

	if err != nil {
		// Rewriting the file failed.

		if n == 0 {
			// Original file unchanged.
			os.Remove(bakname)
			return err
		}

		// Try to restore the original contents.

		no, erro := fout.WriteAt(orig, 0)
		if erro != nil {
			// That failed too.
			return restoreFail(err, erro)
		}

		if no < n {
			// Original file is shorter. Truncate.
			if erro = fout.Truncate(int64(no)); erro != nil {
				return restoreFail(err, erro)
			}
		}

		if erro := fout.Close(); erro != nil {
			return restoreFail(err, erro)
		}

		// Original contents restored.
		os.Remove(bakname)
		return err
	}

	if err := fout.Close(); err != nil {
		return restoreFail(err, err)
	}

	// File updated.
	os.Remove(bakname)
	return nil
}

// backupFile writes data to a new file named filename<number> with permissions perm,
// with <number> randomly chosen such that the file name is unique. backupFile returns
// the chosen file name.
func backupFile(filename string, data []byte, perm fs.FileMode) (string, error) {
	nextRandom := func() string {
		return strconv.Itoa(rand.Int())
	}

	dir, base := filepath.Split(filename)
	var (
		bakname string
		f       *os.File
	)
	for {
		bakname = filepath.Join(dir, base+"."+nextRandom())
		var err error
		f, err = os.OpenFile(bakname, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return "", err
		}
	}

	// write data to backup file
	_, err := f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}
	if err != nil {
		// gocpp: don't leave a part of a backup behind.
		os.Remove(bakname)
		return "", err
	}

	return bakname, err
}
