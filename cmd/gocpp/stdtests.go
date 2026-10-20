package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/stdlib"
)

// stdTestsEnv names the environment variable that adds Go's own test files to the standard
// packages: its value is a folder with one subfolder for each import path (strings/,
// encoding/json/), and the _test.gp files and testdata folders in them join the files of the
// embedded package, so that gocpp test strings runs the tests of the package that gocpp ships
// (tests/stdgo, step 9.10). Without it, the embedded library has no test files.
const stdTestsEnv = "GOCPP_TEST_STDLIB"

// stdlibFS returns the standard library that the loader reads: the embedded one, plus the test
// files of GOCPP_TEST_STDLIB if it is set.
func stdlibFS() fs.FS {
	dir := os.Getenv(stdTestsEnv)
	if dir == "" {
		return stdlib.FS()
	}
	return &testOverlayFS{base: stdlib.FS(), extra: os.DirFS(dir)}
}

// A testOverlayFS is the embedded standard library with the files of another tree added: a file
// of extra with the name of a file of base shadows it, and a folder lists the files of both.
type testOverlayFS struct {
	base  fs.FS
	extra fs.FS
}

func (o *testOverlayFS) Open(name string) (fs.File, error) {
	if f, err := o.extra.Open(name); err == nil {
		info, statErr := f.Stat()
		if statErr == nil && !info.IsDir() {
			return f, nil
		}
		f.Close()
	}
	return o.base.Open(name)
}

func (o *testOverlayFS) ReadFile(name string) ([]byte, error) {
	if data, err := fs.ReadFile(o.extra, name); err == nil {
		return data, nil
	}
	return fs.ReadFile(o.base, name)
}

func (o *testOverlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(o.base, name)
	extra, extraErr := fs.ReadDir(o.extra, name)
	if extraErr != nil {
		if err != nil || !errors.Is(extraErr, fs.ErrNotExist) {
			return entries, err
		}
		return entries, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, entry := range extra {
		replace := slices.IndexFunc(entries, func(e fs.DirEntry) bool { return e.Name() == entry.Name() })
		if replace >= 0 {
			entries[replace] = entry
			continue
		}
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// testRunDir returns the folder a test binary runs in: the folder of the package, as go test does.
// A standard package has no folder (the library is embedded), so its tests run in the folder of
// its test files in GOCPP_TEST_STDLIB (where its testdata is), or else in the current folder.
func testRunDir(dir, importPath string) string {
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	if tests := os.Getenv(stdTestsEnv); tests != "" {
		own := filepath.Join(tests, filepath.FromSlash(importPath))
		if info, err := os.Stat(own); err == nil && info.IsDir() {
			return own
		}
	}
	return ""
}
