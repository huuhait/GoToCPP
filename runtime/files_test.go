package rtfiles_test

import (
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

// skipEntry reports whether the walks of runtime/ in these tests leave out the entry p: a name
// that starts with "." (an editor's or a tool's file: an Emacs lock .#rt.cpp, which is a
// symbolic link that points nowhere, or clangd's .cache/; //go:embed leaves them out too),
// and a symbolic link that points nowhere. For a folder, the walk then skips the whole folder.
func skipEntry(fsys fs.FS, p string, d fs.DirEntry) bool {
	if p != "." && strings.HasPrefix(d.Name(), ".") {
		return true
	}
	if d.Type()&fs.ModeSymlink != 0 {
		if _, err := fs.Stat(fsys, p); err != nil {
			return true
		}
	}
	return false
}

// skipped returns what a WalkDir function returns for an entry that skipEntry leaves out.
func skipped(d fs.DirEntry) error {
	if d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

// TestFSHasEveryFile checks that the embedded FS holds exactly the files of include/ and src/
// on disk. A file that the //go:embed directory patterns skip (a name that starts with "."
// "_") fails here until someone renames it or decides it doesn't ship. Names that start with
// "." are editor and tool files, which this test and //go:embed both leave out.
func TestFSHasEveryFile(t *testing.T) {
	var want []string
	for _, dir := range []string{"include", "src"} {
		disk := os.DirFS(".")
		err := fs.WalkDir(disk, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if skipEntry(disk, p, d) {
				return skipped(d)
			}
			if d.IsDir() {
				return nil
			}
			want = append(want, p)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	err := fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		got = append(got, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the embedded FS holds\n\t%s\nwant the files of include/ and src/\n\t%s\n(files.go embeds the folders include and src; a name that starts with _ is skipped)",
			strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	for _, p := range []string{"include/gocpp/rt.hpp", "include/gocpp/nofortify.h"} {
		if _, err := fs.Stat(rtfiles.FS(), p); err != nil {
			t.Errorf("the embedded FS has no %s: %v", p, err)
		}
	}
}

func TestSources(t *testing.T) {
	linux := rtfiles.Sources("linux")
	if !slices.Contains(linux, "src/rt.cpp") || !slices.Contains(linux, "src/os/linux/os.cpp") {
		t.Errorf(`Sources("linux") = %q, want src/rt.cpp and src/os/linux/os.cpp among them`, linux)
	}
	if !slices.IsSorted(linux) {
		t.Errorf(`Sources("linux") = %q is not sorted`, linux)
	}
	for _, p := range linux {
		if _, err := fs.Stat(rtfiles.FS(), p); err != nil {
			t.Errorf(`Sources("linux") has %s, which is not in FS: %v`, p, err)
		}
	}
	// An OS without an OS layer gets only the portable files.
	for _, p := range rtfiles.Sources("plan9") {
		if strings.HasPrefix(p, "src/os/") {
			t.Errorf(`Sources("plan9") has %s`, p)
		}
	}
}
