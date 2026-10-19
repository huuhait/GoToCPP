// phase: 09
// Go 1.25's io/fs/format_test.go (FormatFileInfo, FormatDirEntry) through a small harness; expected output from real Go.

package main

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"sync/atomic"
	"time"
)


// A MapFS is a simple in-memory file system for use in tests,
// represented as a map from path names (arguments to Open)
// to information about the files, directories, or symbolic links they represent.
//
// The map need not include parent directories for files contained
// in the map; those will be synthesized if needed.
// But a directory can still be included by setting the [MapFile.Mode]'s [fs.ModeDir] bit;
// this may be necessary for detailed control over the directory's [fs.FileInfo]
// or to create an empty directory.
//
// File system operations read directly from the map,
// so that the file system can be changed by editing the map as needed.
// An implication is that file system operations must not run concurrently
// with changes to the map, which would be a race.
// Another implication is that opening or reading a directory requires
// iterating over the entire map, so a MapFS should typically be used with not more
// than a few hundred entries or directory reads.
type MapFS map[string]*MapFile

// A MapFile describes a single file in a [MapFS].
type MapFile struct {
	Data    []byte      // file content or symlink destination
	Mode    fs.FileMode // fs.FileInfo.Mode
	ModTime time.Time   // fs.FileInfo.ModTime
	Sys     any         // fs.FileInfo.Sys
}

var _ fs.FS = MapFS(nil)
var _ fs.ReadLinkFS = MapFS(nil)
var _ fs.File = (*openMapFile)(nil)

// Open opens the named file after following any symbolic links.
func (fsys MapFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	realName, ok := fsys.resolveSymlinks(name)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file := fsys[realName]
	if file != nil && file.Mode&fs.ModeDir == 0 {
		// Ordinary file
		return &openMapFile{name, mapFileInfo{path.Base(name), file}, 0}, nil
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []mapFileInfo
	var need = make(map[string]bool)
	if realName == "." {
		for fname, f := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, mapFileInfo{fname, f})
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		prefix := realName + "/"
		for fname, f := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, mapFileInfo{felem, f})
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if file == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, mapFileInfo{name, &MapFile{Mode: fs.ModeDir | 0555}})
	}
	for i := 1; i < len(list); i++ { // slices.SortFunc(list, by name): package slices is step 7.10
		for j := i; j > 0 && list[j].name < list[j-1].name; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}

	if file == nil {
		file = &MapFile{Mode: fs.ModeDir | 0555}
	}
	var elem string
	if name == "." {
		elem = "."
	} else {
		elem = name[strings.LastIndex(name, "/")+1:]
	}
	return &mapDir{name, mapFileInfo{elem, file}, list, 0}, nil
}

func (fsys MapFS) resolveSymlinks(name string) (_ string, ok bool) {
	// Fast path: if a symlink is in the map, resolve it.
	if file := fsys[name]; file != nil && file.Mode.Type() == fs.ModeSymlink {
		target := string(file.Data)
		if path.IsAbs(target) {
			return "", false
		}
		return fsys.resolveSymlinks(path.Join(path.Dir(name), target))
	}

	// Check if each parent directory (starting at root) is a symlink.
	for i := 0; i < len(name); {
		j := strings.Index(name[i:], "/")
		var dir string
		if j < 0 {
			dir = name
			i = len(name)
		} else {
			dir = name[:i+j]
			i += j
		}
		if file := fsys[dir]; file != nil && file.Mode.Type() == fs.ModeSymlink {
			target := string(file.Data)
			if path.IsAbs(target) {
				return "", false
			}
			return fsys.resolveSymlinks(path.Join(path.Dir(dir), target) + name[i:])
		}
		i += len("/")
	}
	return name, fs.ValidPath(name)
}

// ReadLink returns the destination of the named symbolic link.
func (fsys MapFS) ReadLink(name string) (string, error) {
	info, err := fsys.lstat(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}
	if info.f.Mode.Type() != fs.ModeSymlink {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	return string(info.f.Data), nil
}

// Lstat returns a FileInfo describing the named file.
// If the file is a symbolic link, the returned FileInfo describes the symbolic link.
// Lstat makes no attempt to follow the link.
func (fsys MapFS) Lstat(name string) (fs.FileInfo, error) {
	info, err := fsys.lstat(name)
	if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}
	return info, nil
}

func (fsys MapFS) lstat(name string) (*mapFileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	realDir, ok := fsys.resolveSymlinks(path.Dir(name))
	if !ok {
		return nil, fs.ErrNotExist
	}
	elem := path.Base(name)
	realName := path.Join(realDir, elem)

	file := fsys[realName]
	if file != nil {
		return &mapFileInfo{elem, file}, nil
	}

	if realName == "." {
		return &mapFileInfo{elem, &MapFile{Mode: fs.ModeDir | 0555}}, nil
	}
	// Maybe a directory.
	prefix := realName + "/"
	for fname := range fsys {
		if strings.HasPrefix(fname, prefix) {
			return &mapFileInfo{elem, &MapFile{Mode: fs.ModeDir | 0555}}, nil
		}
	}
	// If the directory name is not in the map,
	// and there are no children of the name in the map,
	// then the directory is treated as not existing.
	return nil, fs.ErrNotExist
}

// fsOnly is a wrapper that hides all but the fs.FS methods,
// to avoid an infinite recursion when implementing special
// methods in terms of helpers that would use them.
// (In general, implementing these methods using the package fs helpers
// is redundant and unnecessary, but having the methods may make
// MapFS exercise more code paths when used in tests.)
type fsOnly struct{ fs.FS }

func (fsys MapFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(fsOnly{fsys}, name)
}

func (fsys MapFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(fsOnly{fsys}, name)
}

func (fsys MapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(fsOnly{fsys}, name)
}

func (fsys MapFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(fsOnly{fsys}, pattern)
}

type noSub struct {
	MapFS
}

func (noSub) Sub() {} // not the fs.SubFS signature

func (fsys MapFS) Sub(dir string) (fs.FS, error) {
	return fs.Sub(noSub{fsys}, dir)
}

// A mapFileInfo implements fs.FileInfo and fs.DirEntry for a given map file.
type mapFileInfo struct {
	name string
	f    *MapFile
}

func (i *mapFileInfo) Name() string               { return path.Base(i.name) }
func (i *mapFileInfo) Size() int64                { return int64(len(i.f.Data)) }
func (i *mapFileInfo) Mode() fs.FileMode          { return i.f.Mode }
func (i *mapFileInfo) Type() fs.FileMode          { return i.f.Mode.Type() }
func (i *mapFileInfo) ModTime() time.Time         { return i.f.ModTime }
func (i *mapFileInfo) IsDir() bool                { return i.f.Mode&fs.ModeDir != 0 }
func (i *mapFileInfo) Sys() any                   { return i.f.Sys }
func (i *mapFileInfo) Info() (fs.FileInfo, error) { return i, nil }

func (i *mapFileInfo) String() string {
	return fs.FormatFileInfo(i)
}

// An openMapFile is a regular (non-directory) fs.File open for reading.
type openMapFile struct {
	path string
	mapFileInfo
	offset int64
}

func (f *openMapFile) Stat() (fs.FileInfo, error) { return &f.mapFileInfo, nil }

func (f *openMapFile) Close() error { return nil }

func (f *openMapFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.f.Data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *openMapFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.f.Data))
	}
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.path, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

func (f *openMapFile) ReadAt(b []byte, offset int64) (int, error) {
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[offset:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

// A mapDir is a directory fs.File (so also an fs.ReadDirFile) open for reading.
type mapDir struct {
	path string
	mapFileInfo
	entry  []mapFileInfo
	offset int
}

func (d *mapDir) Stat() (fs.FileInfo, error) { return &d.mapFileInfo, nil }
func (d *mapDir) Close() error               { return nil }
func (d *mapDir) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *mapDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}
var testFsys = MapFS{
	"hello.txt": {
		Data:    []byte("hello, world"),
		Mode:    0456,
		ModTime: time.Now(),
		Sys:     &sysValue,
	},
	"sub/goodbye.txt": {
		Data:    []byte("goodbye, world"),
		Mode:    0456,
		ModTime: time.Now(),
		Sys:     &sysValue,
	},
}

var sysValue int

type readFileOnly struct{ fs.ReadFileFS }

func (readFileOnly) Open(name string) (fs.File, error) { return nil, fs.ErrNotExist }

type openOnly struct{ fs.FS }

// formatTest implements FileInfo to test FormatFileInfo,
// and implements DirEntry to test FormatDirEntry.
type formatTest struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fs *formatTest) Name() string {
	return fs.name
}

func (fs *formatTest) Size() int64 {
	return fs.size
}

func (fs *formatTest) Mode() fs.FileMode {
	return fs.mode
}

func (fs *formatTest) ModTime() time.Time {
	return fs.modTime
}

func (fs *formatTest) IsDir() bool {
	return fs.isDir
}

func (fs *formatTest) Sys() any {
	return nil
}

func (fs *formatTest) Type() fs.FileMode {
	return fs.mode.Type()
}

func (fs *formatTest) Info() (fs.FileInfo, error) {
	return fs, nil
}

var formatTests = []struct {
	input        formatTest
	wantFileInfo string
	wantDirEntry string
}{
	{
		formatTest{
			name:    "hello.go",
			size:    100,
			mode:    0o644,
			modTime: time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC),
			isDir:   false,
		},
		"-rw-r--r-- 100 1970-01-01 12:00:00 hello.go",
		"- hello.go",
	},
	{
		formatTest{
			name:    "home/gopher",
			size:    0,
			mode:    fs.ModeDir | 0o755,
			modTime: time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC),
			isDir:   true,
		},
		"drwxr-xr-x 0 1970-01-01 12:00:00 home/gopher/",
		"d home/gopher/",
	},
	{
		formatTest{
			name:    "big",
			size:    0x7fffffffffffffff,
			mode:    fs.ModeIrregular | 0o644,
			modTime: time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC),
			isDir:   false,
		},
		"?rw-r--r-- 9223372036854775807 1970-01-01 12:00:00 big",
		"? big",
	},
	{
		formatTest{
			name:    "small",
			size:    -0x8000000000000000,
			mode:    fs.ModeSocket | fs.ModeSetuid | 0o644,
			modTime: time.Date(1970, time.January, 1, 12, 0, 0, 0, time.UTC),
			isDir:   false,
		},
		"Surw-r--r-- -9223372036854775808 1970-01-01 12:00:00 small",
		"S small",
	},
}

func TestFormatFileInfo(t *testT) {
	for i, test := range formatTests {
		got := fs.FormatFileInfo(&test.input)
		atomic.AddInt64(&checks, 1)
		if got != test.wantFileInfo {
			t.Errorf("%d: FormatFileInfo(%#v) = %q, want %q", i, test.input, got, test.wantFileInfo)
		}
	}
}

func TestFormatDirEntry(t *testT) {
	for i, test := range formatTests {
		got := fs.FormatDirEntry(&test.input)
		atomic.AddInt64(&checks, 1)
		if got != test.wantDirEntry {
			t.Errorf("%d: FormatDirEntry(%#v) = %q, want %q", i, test.input, got, test.wantDirEntry)
		}
	}

}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
// It is atomic: some tests run their checks in several goroutines.
var checks int64

func (t *testT) Helper()                          {}
func (t *testT) Parallel()                        {}
func (t *testT) Name() string                     { return t.name }
func (t *testT) Log(args ...any)                  {}
func (t *testT) Logf(format string, args ...any)  {}
func (t *testT) Skip(args ...any)                 { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)      { panic(skipExit{}) }
func (t *testT) report(msg string)                { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		atomic.StoreInt64(&checks, 0)
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", atomic.LoadInt64(&checks))
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// allocsPerRun runs f runs times and returns 0: gocpp does not count heap allocations (the tests of allocation counts are
// dropped or their comparison is).
func allocsPerRun(runs int, f func()) float64 {
	for i := 0; i < runs; i++ {
		f()
	}
	return 0
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32 { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestFormatFileInfo", TestFormatFileInfo},
		{"TestFormatDirEntry", TestFormatDirEntry},
	}
	runTests(tests_)
}

// Output:
// ok TestFormatFileInfo subtests: 0 checks: 4
// ok TestFormatDirEntry subtests: 0 checks: 4
// tests: 2 failed: 0
