// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os_test

import (
	"errors"
	"flag"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testMaybeRooted calls f in two subtests,
// one with a Root and one with a nil r.
func testMaybeRooted(t *testing.T, f func(t *testing.T, r *os.Root)) {
	t.Run("NoRoot", func(t *testing.T) {
		t.Chdir(t.TempDir())
		f(t, nil)
	})
}

// makefs creates a test filesystem layout and returns the path to its root.
//
// Each entry in the slice is a file, directory, or symbolic link to create:
//
//   - "d/": directory d
//   - "f": file f with contents f
//   - "a => b": symlink a with target b
//
// The directory containing the filesystem is always named ROOT.
// $ABS is replaced with the absolute path of the directory containing the filesystem.
//
// Parent directories are automatically created as needed.
//
// makefs calls t.Skip if the layout contains features not supported by the current GOOS.
func makefs(t *testing.T, fs []string) string {
	root := filepath.Join(t.TempDir(), "ROOT")
	if err := os.Mkdir(root, 0o777); err != nil {
		t.Fatal(err)
	}
	for _, ent := range fs {
		ent = strings.ReplaceAll(ent, "$ABS", root)
		base, link, isLink := strings.Cut(ent, " => ")
		if isLink {
			if runtime.GOOS == "wasip1" && path.IsAbs(link) {
				t.Skip("absolute link targets not supported on " + runtime.GOOS)
			}
			if runtime.GOOS == "plan9" {
				t.Skip("symlinks not supported on " + runtime.GOOS)
			}
			ent = base
		}
		if err := os.MkdirAll(path.Join(root, path.Dir(base)), 0o777); err != nil {
			t.Fatal(err)
		}
		if isLink {
			if err := os.Symlink(link, path.Join(root, base)); err != nil {
				t.Fatal(err)
			}
		} else if strings.HasSuffix(ent, "/") {
			if err := os.MkdirAll(path.Join(root, ent), 0o777); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(path.Join(root, ent), []byte(ent), 0o666); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// A rootTest is a test case for os.Root.
type rootTest struct {
	name string

	// fs is the test filesystem layout. See makefs above.
	fs []string

	// open is the filename to access in the test.
	open string

	// target is the filename that we expect to be accessed, after resolving all symlinks.
	// For test cases where the operation fails due to an escaping path such as ../ROOT/x,
	// the target is the filename that should not have been opened.
	target string

	// ltarget is the filename that we expect to accessed, after resolving all symlinks
	// except the last one. This is the file we expect to be removed by Remove or statted
	// by Lstat.
	//
	// If the last path component in open is not a symlink, ltarget should be "".
	ltarget string

	// wantError is true if accessing the file should fail.
	wantError bool

	// alwaysFails is true if the open operation is expected to fail
	// even when using non-openat operations.
	//
	// This lets us check that tests that are expected to fail because (for example)
	// a path escapes the directory root will succeed when the escaping checks are not
	// performed.
	alwaysFails bool
}



var rootTestCases = []rootTest{{
	name:   "plain path",
	fs:     []string{},
	open:   "target",
	target: "target",
}, {
	name: "path in directory",
	fs: []string{
		"a/b/c/",
	},
	open:   "a/b/c/target",
	target: "a/b/c/target",
}, {
	name: "symlink",
	fs: []string{
		"link => target",
	},
	open:    "link",
	target:  "target",
	ltarget: "link",
}, {
	name: "symlink dotdot slash",
	fs: []string{
		"link => ../",
	},
	open:      "link",
	ltarget:   "link",
	wantError: true,
}, {
	name: "symlink ending in slash",
	fs: []string{
		"dir/",
		"link => dir/",
	},
	open:   "link/target",
	target: "dir/target",
}, {
	name: "slash after symlink to file",
	fs: []string{
		"link => ../ROOT/target",
	},
	open:      "link/",
	target:    "target",
	wantError: true,
}, {
	name: "slash after symlink to dir",
	fs: []string{
		"link => ../ROOT/target",
		"target/",
	},
	open:      "link/",
	wantError: true,
}, {
	name: "symlink dotdot dotdot slash",
	fs: []string{
		"dir/link => ../../",
	},
	open:      "dir/link",
	ltarget:   "dir/link",
	wantError: true,
}, {
	name: "symlink chain",
	fs: []string{
		"link => a/b/c/target",
		"a/b => e",
		"a/e => ../f",
		"f => g/h/i",
		"g/h/i => ..",
		"g/c/",
	},
	open:    "link",
	target:  "g/c/target",
	ltarget: "link",
}, {
	name: "path with dot",
	fs: []string{
		"a/b/",
	},
	open:   "./a/./b/./target",
	target: "a/b/target",
}, {
	name: "path with dotdot",
	fs: []string{
		"a/b/",
	},
	open:   "a/../a/b/../../a/b/../b/target",
	target: "a/b/target",
}, {
	name:      "path with dotdot slash",
	fs:        []string{},
	open:      "../",
	wantError: true,
}, {
	name: "path with dotdot dotdot slash",
	fs: []string{
		"a/",
	},
	open:      "a/../../",
	wantError: true,
}, {
	name: "dotdot no symlink",
	fs: []string{
		"a/",
	},
	open:   "a/../target",
	target: "target",
}, {
	name: "dotdot after symlink",
	fs: []string{
		"a => b/c",
		"b/c/",
	},
	open: "a/../target",
	target: func() string {
		if runtime.GOOS == "windows" {
			// On Windows, the path is cleaned before symlink resolution.
			return "target"
		}
		return "b/target"
	}(),
}, {
	name: "dotdot before symlink",
	fs: []string{
		"a => b/c",
		"b/c/",
	},
	open:   "b/../a/target",
	target: "b/c/target",
}, {
	name: "symlink ends in dot",
	fs: []string{
		"a => b/.",
		"b/",
	},
	open:   "a/target",
	target: "b/target",
}, {
	name:        "directory does not exist",
	fs:          []string{},
	open:        "a/file",
	wantError:   true,
	alwaysFails: true,
}, {
	name:        "empty path",
	fs:          []string{},
	open:        "",
	wantError:   true,
	alwaysFails: true,
}, {
	name: "symlink cycle",
	fs: []string{
		"a => a",
	},
	open:        "a",
	ltarget:     "a",
	wantError:   true,
	alwaysFails: true,
}, {
	name:      "path escapes",
	fs:        []string{},
	open:      "../ROOT/target",
	target:    "target",
	wantError: true,
}, {
	name: "long path escapes",
	fs: []string{
		"a/",
	},
	open:      "a/../../ROOT/target",
	target:    "target",
	wantError: true,
}, {
	name: "absolute symlink",
	fs: []string{
		"link => $ABS/target",
	},
	open:      "link",
	ltarget:   "link",
	target:    "target",
	wantError: true,
}, {
	name: "relative symlink",
	fs: []string{
		"link => ../ROOT/target",
	},
	open:      "link",
	target:    "target",
	ltarget:   "link",
	wantError: true,
}, {
	name: "symlink chain escapes",
	fs: []string{
		"link => a/b/c/target",
		"a/b => e",
		"a/e => ../../ROOT",
		"c/",
	},
	open:      "link",
	target:    "c/target",
	ltarget:   "link",
	wantError: true,
}}























// A rootConsistencyTest is a test case comparing os.Root behavior with
// the corresponding non-Root function.
//
// These tests verify that, for example, Root.Open("file/./") and os.Open("file/./")
// have the same result, although the specific result may vary by platform.
type rootConsistencyTest struct {
	name string

	// fs is the test filesystem layout. See makefs above.
	// fsFunc is called to modify the test filesystem, or replace it.
	fs     []string
	fsFunc func(t *testing.T, dir string) string

	// open is the filename to access in the test.
	open string

	// detailedErrorMismatch indicates that os.Root and the corresponding non-Root
	// function return different errors for this test.
	detailedErrorMismatch func(t *testing.T) bool

	// check is called before the test starts, and may t.Skip if necessary.
	check func(t *testing.T)
}

var rootConsistencyTestCases = []rootConsistencyTest{{
	name: "file",
	fs: []string{
		"target",
	},
	open: "target",
}, {
	name: "dir slash dot",
	fs: []string{
		"target/file",
	},
	open: "target/.",
}, {
	name: "dot",
	fs: []string{
		"file",
	},
	open: ".",
}, {
	name: "file slash dot",
	fs: []string{
		"target",
	},
	open: "target/.",
	detailedErrorMismatch: func(t *testing.T) bool {
		// FreeBSD returns EPERM in the non-Root case.
		return runtime.GOOS == "freebsd" && strings.HasPrefix(t.Name(), "TestRootConsistencyRemove")
	},
}, {
	name: "dir slash",
	fs: []string{
		"target/file",
	},
	open: "target/",
}, {
	name: "dot slash",
	fs: []string{
		"file",
	},
	open: "./",
}, {
	name: "file slash",
	fs: []string{
		"target",
	},
	open: "target/",
	detailedErrorMismatch: func(t *testing.T) bool {
		// os.Create returns ENOTDIR or EISDIR depending on the platform.
		return runtime.GOOS == "js"
	},
}, {
	name: "file in path",
	fs: []string{
		"file",
	},
	open: "file/target",
}, {
	name: "directory in path missing",
	open: "dir/target",
}, {
	name: "target does not exist",
	open: "target",
}, {
	name: "symlink slash",
	fs: []string{
		"target/file",
		"link => target",
	},
	open: "link/",
	check: func(t *testing.T) {
		if runtime.GOOS == "linux" && strings.HasPrefix(t.Name(), "TestRootConsistencyRename/") {
			// Linux does not resolve "symlink" in rename("symlink/", "target").
			t.Skip("known inconsistency on linux")
		}
		if strings.HasPrefix(t.Name(), "TestRootConsistencyRemoveAll/") {
			// Root.RemoveAll and os.RemoveAll are not always consistent here.
			t.Skip("known inconsistency in RemoveAll")
		}
	},
}, {
	name: "symlink slash dot",
	fs: []string{
		"target/file",
		"link => target",
	},
	open: "link/.",
}, {
	name: "unresolved symlink",
	fs: []string{
		"link => target",
	},
	open: "link",
}, {
	name: "resolved symlink",
	fs: []string{
		"link => target",
		"target",
	},
	open: "link",
}, {
	name: "dotdot in path after symlink",
	fs: []string{
		"a => b/c",
		"b/c/",
		"b/target",
	},
	open: "a/../target",
}, {
	name: "symlink to dir ends in slash",
	fs: []string{
		"dir/",
		"link => dir/",
	},
	open: "link",
}, {
	name: "symlink to file ends in slash",
	fs: []string{
		"file",
		"link => file/",
	},
	open: "link",
}, {
	name: "long file name",
	open: strings.Repeat("a", 500),
}, {
	name: "unreadable directory",
	fs: []string{
		"dir/target",
	},
	fsFunc: func(t *testing.T, dir string) string {
		os.Chmod(filepath.Join(dir, "dir"), 0)
		t.Cleanup(func() {
			os.Chmod(filepath.Join(dir, "dir"), 0o700)
		})
		return dir
	},
	open: "dir/target",
}, {
	name: "unix domain socket target",
	fsFunc: func(t *testing.T, dir string) string {
		return tempDirWithUnixSocket(t, "a")
	},
	open: "a",
}, {
	name: "unix domain socket in path",
	fsFunc: func(t *testing.T, dir string) string {
		return tempDirWithUnixSocket(t, "a")
	},
	open: "a/b",
	detailedErrorMismatch: func(t *testing.T) bool {
		// On Windows, os.Root.Open returns "The directory name is invalid."
		// and os.Open returns "The file cannot be accessed by the system.".
		return runtime.GOOS == "windows"
	},
	check: func(t *testing.T) {
		if strings.HasPrefix(t.Name(), "TestRootConsistencyRemoveAll/") {
			switch runtime.GOOS {
			case "windows":
				// Root.RemoveAll notices that a/ is not a directory,
				// and returns success.
				// os.RemoveAll tries to open a/ and fails because
				// it is not a regular file.
				// The inconsistency here isn't worth fixing, so just skip this test.
				t.Skip("known inconsistency on windows")
			case "js":
				// GOOS=js behavior varies with what the underlying OS is.
				t.Skip("known inconsistency with GOOS=js")
			}
		}
	},
}, {
	name: "question mark",
	open: "?",
}, {
	name: "nul byte",
	open: "\x00",
}}

func tempDirWithUnixSocket(t *testing.T, name string) string {
	t.Skip("it needs package net for a unix socket, which gocpp does not have")
	return ""
}



























// A rootMultiTest is state for testing an os.Root operation in one configuration among many.
// Each execution of a rootMultiTest varies in several ways:
//
//   - With or without an *os.Root, to check consistency between root/non-root operations.
//   - With a target that may be a file, directory, symlink, or entirely absent.
//   - With various paths referencing the target: "target", "DIR/../target", etc.
//   - When the target is a symlink, with various link target paths.
//
// For example, a single test execution might be:
// In an *os.Root, copy "source" to "DIR/../target".
// "source" is a file, and "target" is a symlink to "../ROOT/s_target". "s_target" is a directory.
// (In this case, we expect the test to fail due to the path escape in the symlink.)
type rootMultiTest struct {
	// dir is the directory containing the test.
	// dir will always contain a directory named "ROOT"
	// and a subdir named "ROOT/DIR".
	dir string

	// root is the *Root for the test. May be nil.
	root *os.Root

	// source and target are files acted on by the test.
	// target is always set; source is only set for tests which request two files.
	source testFileDesc
	target testFileDesc

	// sourcePath and targetPath are the paths which should be used to acceess
	// the source/target.
	sourcePath string
	targetPath string

	sourceInfo os.FileInfo
	targetInfo os.FileInfo

	// op is the operation being performed, used for reporting errors.
	op string
}

var testVerbose = flag.Bool("verbose", false, "verbose")

// A rootMultiTest function may return this error to disable
// the check that in-root and out-of-root functions have the same outcome.
var errSkipRootConsistencyCheck = errors.New("skip root consistency check")




var errAny = errors.New("any error")





// testFileKind is a kind of file.
type testFileKind int

const (
	testFileUnused  = testFileKind(iota)
	testFileAbsent  // file does not exist
	testFileFile    // regular file
	testFileDir     // directory
	testFileSymlink // symlink
	testFileMax

	// testFileError represents a path which fails during resolution,
	// such as "a/b" where "a" does not exist.
	testFileError
)


// testFileRef is a kind of reference to a file.
//
// Many path names can refer to the same file: f, ./f, /abs/path/to/f, somedir/../f, etc.
// A testFileRef describes some form of reference.
type testFileRef struct {
	// name is the name of the reference (not the file name).
	// These are a bit cryptic to keep test names short:
	// s (/ slash), p (.. parent), b (base), d (directory), r (root)
	name string

	// template is a template path.
	//
	// templates assume that the file is contained in a directory named "ROOT",
	// and that "ROOT/DIR" exists and is a directory.
	//
	// The string BASE in the template may be replaced with the file's basename.
	//
	// Absolute path templates start with /ROOT.
	template string

	// escapes indicates whether the path escapes the current directory.
	escapes bool
}

var testFileRefs = []testFileRef{
	{escapes: false, name: "b", template: "BASE"},
	{escapes: false, name: "bs", template: "BASE/"},
	{escapes: false, name: "dpb", template: "DIR/../BASE"},
	{escapes: false, name: "dpbs", template: "DIR/../BASE/"},
	{escapes: true, name: "prb", template: "../ROOT/BASE"},
	{escapes: true, name: "prbs", template: "../ROOT/BASE/"},
	{escapes: true, name: "srb", template: "/ROOT/BASE"},
	{escapes: true, name: "srbs", template: "/ROOT/BASE/"},
}

// testFileLimitedRefs is a smaller set of references which do not exercise path escapes
// (see allTestFileDescs).
var testFileLimitedRefs = testFileRefs[0:2]



// testFileDesc is a description of a type of file, combining the kind and reference type.
//
// Some sample testFileDescs:
//   - "name", a plain file.
//   - "DIR/../name", a directory
//   - "name/", where name is a symlink to "DIR/../target/", where target is a plain file.
type testFileDesc struct {
	kind   testFileKind
	ref    testFileRef
	target *testFileDesc // symlink target, nil when kind is not testFileSymlink
}

var rootComprehensive = flag.Bool("root_comprehensive", false,
	"run many more os.Root test variations (slow, uncertain value)")































