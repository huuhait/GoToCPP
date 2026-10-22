//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// gocpp export compdb -o names a file or a folder like go build -o: a name with a trailing slash
// is a folder (made when it is missing), and a named pipe is refused instead of waited for.
func TestExportCompdbTargets(t *testing.T) {
	dir := compdbSetup(t, "target")
	out := t.TempDir()

	t.Run("new folder with a slash", func(t *testing.T) {
		name := filepath.Join(out, "newdir") + "/"
		r := gocppRun("", false, "export", "compdb", "-o", name, ".")
		want := filepath.Join(out, "newdir", "compile_commands.json")
		if r.code != 0 || strings.TrimSpace(r.stdout) != want || len(readCompDB(t, want)) == 0 {
			t.Errorf("export compdb -o %s = %+v, want the file %s", name, r, want)
		}
	})

	t.Run("named pipe", func(t *testing.T) {
		pipe := filepath.Join(out, "pipe")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { // lets a stuck open go on: no goroutine outlives the test
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		})
		done := make(chan result, 1)
		go func() { done <- gocppRun("", false, "export", "compdb", "-o", pipe, ".") }()
		select {
		case r := <-done:
			if r.code == 0 || !strings.Contains(r.stderr, "is not a regular file (a named pipe)") {
				t.Errorf("export compdb -o pipe = %+v, want an error about the named pipe", r)
			}
		case <-time.After(time.Minute):
			t.Fatal("export compdb -o pipe waits for a writer (no result after a minute)")
		}
	})

	t.Run("link to a folder", func(t *testing.T) {
		behind := filepath.Join(out, "behind")
		if err := os.Mkdir(behind, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(out, "folderlink")
		if err := os.Symlink(behind, link); err != nil {
			t.Skip(err)
		}
		r := gocppRun("", false, "export", "compdb", "-o", link, ".")
		if r.code == 0 || !strings.Contains(r.stderr, "symbolic link to a folder") ||
			exists(filepath.Join(behind, "compile_commands.json")) {
			t.Errorf("export compdb -o link-to-folder = %+v, want the error and nothing written behind it", r)
		}
	})

	t.Run("module folder is a link to a folder", func(t *testing.T) {
		// The automatic write of gocpp build names the file, so a link to a folder with the name
		// of the file is left alone (no write behind it).
		behind := filepath.Join(out, "autobehind")
		if err := os.Mkdir(behind, 0o755); err != nil {
			t.Fatal(err)
		}
		db := filepath.Join(dir, "compile_commands.json")
		os.Remove(db)
		if err := os.Symlink(behind, db); err != nil {
			t.Skip(err)
		}
		r := gocppRun("", false, "build", "-o", "prog", ".")
		if r.code != 0 || exists(filepath.Join(behind, "compile_commands.json")) {
			t.Errorf("gocpp build = %+v, want a build that writes nothing behind the link", r)
		}
	})
}

// TestExportCompdbRelativeNames: a name with a trailing slash is a folder when it is relative to
// the current folder too (filepath.Join dropped the slash, so `-o newdir/` wrote a file newdir),
// and every kind of name gives the same answer spelled absolute or relative.
func TestExportCompdbRelativeNames(t *testing.T) {
	dir := compdbSetup(t, "relname")
	behind := filepath.Join(dir, "behind")
	for _, folder := range []string{"bin", "behind"} {
		if err := os.Mkdir(filepath.Join(dir, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(behind, filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		want string // the file it writes, relative to dir; "" when it is an error
		err  string // the text of the error
	}{
		{name: "newdir/", want: "newdir/compile_commands.json"},
		{name: "new/sub/", want: "new/sub/compile_commands.json"},
		{name: "bin/", want: "bin/compile_commands.json"},
		{name: "bin", want: "bin/compile_commands.json"},
		{name: "link/", want: "behind/compile_commands.json"},
		{name: "link", err: "symbolic link to a folder"},
		{name: "file/", err: "is a file, not a folder"},
		{name: "plain.json", want: "plain.json"},
	}
	for _, spelling := range []string{"relative", "absolute"} {
		for _, c := range cases {
			t.Run(spelling+"/"+c.name, func(t *testing.T) {
				arg := c.name
				if spelling == "absolute" {
					arg = filepath.Join(dir, c.name)
					if strings.HasSuffix(c.name, "/") {
						arg += "/"
					}
				}
				r := gocppRun("", false, "export", "compdb", "-o", arg, ".")
				if c.err != "" {
					if r.code == 0 || !strings.Contains(r.stderr, c.err) {
						t.Errorf("export compdb -o %s = %+v, want the error %q", arg, r, c.err)
					}
					return
				}
				want := filepath.Join(dir, filepath.FromSlash(c.want))
				if r.code != 0 || !exists(want) {
					t.Fatalf("export compdb -o %s = %+v, want the file %s", arg, r, want)
				}
				if len(readCompDB(t, want)) == 0 {
					t.Errorf("%s has no commands", want)
				}
			})
		}
	}
}

// TestBuildCompdbRelativeFolder: gocpp build -compdb=folder/ names a folder, relative or not.
func TestBuildCompdbRelativeFolder(t *testing.T) {
	dir := compdbSetup(t, "buildrel")
	r := gocppRun("", false, "build", "-compdb=dbdir/", "-o", "prog", ".")
	want := filepath.Join(dir, "dbdir", "compile_commands.json")
	if r.code != 0 || !exists(want) {
		t.Errorf("gocpp build -compdb=dbdir/ = %+v, want the file %s", r, want)
	}
}
