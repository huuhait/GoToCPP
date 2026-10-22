package load

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// loadGuarded loads the patterns in dir like loadIn, but fails the test instead of hanging when
// the load waits for a named pipe: a load that makes no progress for a minute is a hang. The
// cleanup opens the pipes for writing once, which lets a stuck open go on, so no goroutine
// outlives the test.
func loadGuarded(t *testing.T, root string, pipes []string, patterns ...string) *Program {
	t.Helper()
	t.Cleanup(func() {
		for _, pipe := range pipes {
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		}
	})
	done := make(chan *Program, 1)
	go func() {
		cfg := Config{Dir: root, Stdlib: fakeStd, GoStd: goStd, GOOS: "linux", GOARCH: "amd64"}
		done <- Load(cfg, patterns...)
	}()
	select {
	case prog := <-done:
		checkProgram(t, prog)
		return prog
	case <-time.After(time.Minute):
		t.Fatalf("the load waits for a named pipe (no result after a minute)")
		return nil
	}
}

// TestIrregularSources: a named pipe, or a link to one, with the name of a source or of a module
// file is an error that says what it is; the load does not wait for a writer. A folder with the
// name of a source is left alone, as before.
func TestIrregularSources(t *testing.T) {
	base := map[string]string{
		"gocpp.mod": "module m\n",
		"main.gp":   "package main\n\nfunc main() {}\n",
	}
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) (pipes []string)
		want  string // in the errors, "" for none
	}{
		{"gp pipe", fifoAt("pipe.gp"), "pipe.gp: open "},
		{"gp pipe in a test file", fifoAt("pipe_test.gp"), ""},
		{"gp link to a pipe", fifoLinkAt("pipe.gp"), "pipe.gp: open "},
		{"c pipe", fifoAt("a.c"), "a.c: open "},
		{"cpp pipe", fifoAt("a.cpp"), "a.cpp: open "},
		{"cc link to a pipe", fifoLinkAt("a.cc"), "a.cc: open "},
		{"cxx pipe", fifoAt("a.cxx"), "a.cxx: open "},
		{"header pipes are not sources", func(t *testing.T, root string) []string {
			return append(fifoAt("a.h")(t, root), fifoAt("a.hpp")(t, root)...)
		}, ""},
		{"folder named like a source", func(t *testing.T, root string) []string {
			if err := os.Mkdir(filepath.Join(root, "dir.gp"), 0o755); err != nil {
				t.Fatal(err)
			}
			return nil
		}, ""},
		{"module file pipe", func(t *testing.T, root string) []string {
			path := filepath.Join(root, "gocpp.mod")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			return fifoAt("gocpp.mod")(t, root)
		}, "gocpp.mod: open "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t, base)
			pipes := tc.setup(t, root)
			prog := loadGuarded(t, root, pipes, ".")
			got := errorText(t, prog, root)
			if tc.want == "" {
				if strings.Contains(got, "named pipe") {
					t.Errorf("errors:\n%s\nwant no error about a pipe", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) || !strings.Contains(got, "not a regular file: it is a named pipe") {
				t.Errorf("errors:\n%s\nwant %q and the kind of file", got, tc.want)
			}
		})
	}
}

// fifoAt makes a named pipe with the name in the folder of the test (it skips the test where
// there are none) and returns it.
func fifoAt(name string) func(t *testing.T, root string) []string {
	return func(t *testing.T, root string) []string {
		path := filepath.Join(root, name)
		if err := mkfifo(path); err != nil {
			t.Skip(err)
		}
		return []string{path}
	}
}

// fifoLinkAt makes a symbolic link with the name to a named pipe, which is outside the folder of
// the package.
func fifoLinkAt(name string) func(t *testing.T, root string) []string {
	return func(t *testing.T, root string) []string {
		target := filepath.Join(t.TempDir(), "pipe")
		if err := mkfifo(target); err != nil {
			t.Skip(err)
		}
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skip(err)
		}
		return []string{target}
	}
}

// TestReadRegular: ReadRegular reads a file and a link to a file, and refuses a pipe and a
// device without waiting.
func TestReadRegular(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRegular(file)
	if err != nil || string(got) != "text" {
		t.Errorf("ReadRegular(file) = %q, %v", got, err)
	}
	if _, err := ReadRegular(dir); err == nil || !strings.Contains(err.Error(), "it is a folder") {
		t.Errorf("ReadRegular(folder) = %v, want a folder", err)
	}
	if _, err := ReadRegular("/dev/zero"); err == nil && filepath.Separator == '/' {
		t.Errorf("/dev/zero was read")
	}
}

// TestDirFS: DirFS reads files and lists folders like os.DirFS, and a named pipe cannot be opened
// (open and the ReadFile of the fs package say what it is).
func TestDirFS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.gp"), []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	pipes := mkfifo(filepath.Join(dir, "pipe.gp")) == nil
	fsys := DirFS(dir)
	if got, err := fs.ReadFile(fsys, "a.gp"); err != nil || string(got) != "text" {
		t.Errorf("ReadFile(a.gp) = %q, %v", got, err)
	}
	wantEntries := 1
	if pipes {
		wantEntries = 2
	}
	if entries, err := fs.ReadDir(fsys, "."); err != nil || len(entries) != wantEntries {
		t.Errorf("ReadDir = %v, %v; want %d entries", entries, err, wantEntries)
	}
	if fi, err := fs.Stat(fsys, "."); err != nil || !fi.IsDir() || fi.Name() != "." {
		t.Errorf("Stat(.) = %v, %v; want the folder named .", fi, err)
	}
	if pipes {
		if _, err := fsys.Open("pipe.gp"); err == nil || !strings.Contains(err.Error(), "it is a named pipe") {
			t.Errorf("Open(pipe.gp) = %v, want that it is a named pipe", err)
		}
		if _, err := fs.ReadFile(fsys, "pipe.gp"); err == nil || !strings.Contains(err.Error(), "it is a named pipe") {
			t.Errorf("ReadFile(pipe.gp) = %v, want that it is a named pipe", err)
		}
	}
	var pathErr *fs.PathError
	if _, err := fs.ReadFile(fsys, "missing.gp"); !errors.As(err, &pathErr) || pathErr.Path != "missing.gp" || pathErr.Op != "readfile" {
		t.Errorf("ReadFile(missing.gp) = %v, want a PathError for readfile missing.gp", err)
	}
	plain := t.TempDir()
	if err := os.WriteFile(filepath.Join(plain, "a.gp"), []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(DirFS(plain), "a.gp"); err != nil {
		t.Error(err)
	}
}

// TestDirFSInvalidNames: a name that is not a slash path inside the folder is invalid, as in
// os.DirFS: dots that leave the folder, a NUL, and on Windows a backslash or a colon (which would
// turn "..\\secret.txt" or "C:\\x" into a path outside the folder).
func TestDirFSInvalidNames(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dir")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ok.txt", "secret.txt"} {
		path := filepath.Join(root, name)
		if name == "ok.txt" {
			path = filepath.Join(dir, name)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names := []string{"../secret.txt", "sub/../../secret.txt", "/ok.txt", "ok.txt/", "a\x00b"}
	if filepath.Separator != '/' {
		names = append(names, `..\secret.txt`, `sub\..\..\secret.txt`, `\ok.txt`, `sub\ok.txt`, `ok.txt:stream`, `C:\x`, filepath.Join(root, "secret.txt"))
	}
	fsys := DirFS(dir)
	for _, name := range names {
		if _, err := fsys.Open(name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Open(%q) = %v, want an invalid argument", name, err)
		}
		if _, err := fs.ReadFile(fsys, name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("ReadFile(%q) = %v, want an invalid argument", name, err)
		}
		if _, err := fs.Stat(fsys, name); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Stat(%q) = %v, want an invalid argument", name, err)
		}
	}
}
