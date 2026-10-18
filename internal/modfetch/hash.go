package modfetch

// This file is adapted from golang.org/x/mod/sumdb/dirhash:
//
//	Copyright 2018 The Go Authors. All rights reserved.
//	Use of this source code is governed by a BSD-style
//	license that can be found in the LICENSE file of the Go distribution.

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Hash1 is the "h1:" hash of a list of files: "h1:" and the base64 of the
// SHA-256 of the text of a line for each file, in the order of the sorted
// names, which is the hex SHA-256 of the content, two spaces, the name and a
// newline. It is the hash that go.sum has for a module, so for the same files
// the hash is the same as Go's. A name may not have a newline.
func Hash1(files []string, open func(string) (io.ReadCloser, error)) (string, error) {
	h := sha256.New()
	files = append([]string(nil), files...)
	slices.Sort(files)
	for _, file := range files {
		if strings.Contains(file, "\n") {
			return "", errors.New("file names with newlines are not supported")
		}
		r, err := open(file)
		if err != nil {
			return "", err
		}
		hf := sha256.New()
		_, err = io.Copy(hf, r)
		r.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%x  %s\n", hf.Sum(nil), file)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// HashDir returns the "h1:" hash of the files in the tree dir, with the names
// path@version/<name relative to dir>, as go.sum does for a module. Only
// regular files count: a link, or anything else that is not a file or a
// folder, is an error, since a module never has one. Empty folders and file
// modes do not count.
func HashDir(ctx context.Context, dir, path, version string) (string, error) {
	prefix := path + "@" + version
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files = append(files, prefix+"/"+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	return Hash1(files, func(name string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(name, prefix+"/"))))
	})
}

// HashFile returns the "h1:" hash of one file, named name in the hash, which
// is how go.sum hashes go.mod (here gocpp.mod).
func HashFile(file, name string) (string, error) {
	return Hash1([]string{name}, func(string) (io.ReadCloser, error) { return os.Open(file) })
}

// ModFileName is the name of the module file in the module hash line of
// gocpp.sum: "path version/gocpp.mod h1:...".
const ModFileName = "gocpp.mod"
