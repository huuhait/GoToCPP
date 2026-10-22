//go:build unix

package build

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeCompDBGuarded is WriteCompDB that fails the test instead of hanging when the write waits
// for a named pipe: no result for a minute is a hang. The cleanup opens the pipes once, so that a
// stuck open goes on and no goroutine outlives the test.
func writeCompDBGuarded(t *testing.T, name string, pipes ...string) (string, error) {
	t.Helper()
	t.Cleanup(func() {
		for _, pipe := range pipes {
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		}
	})
	type result struct {
		path string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		path, err := WriteCompDB(testPlan(), name)
		done <- result{path, err}
	}()
	select {
	case r := <-done:
		return r.path, r.err
	case <-time.After(time.Minute):
		t.Fatalf("WriteCompDB(%s) waits for a named pipe (no result after a minute)", name)
		return "", nil
	}
}

// A named pipe, or a socket or a device, is not a target: the write says so and does not open
// it (it waited for a writer for ever and ignored SIGTERM). A link to a pipe is replaced by the
// file, as every link is, and the pipe is not touched.
func TestWriteCompDBRefusesIrregularTargets(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	// A socket path is short: the folder of the test may be too long for one.
	sock := filepath.Join(t.TempDir(), "s")
	if lis, err := net.Listen("unix", sock); err == nil {
		t.Cleanup(func() { lis.Close() })
	} else {
		sock = ""
	}
	targets := map[string]string{"named pipe": pipe, "device": "/dev/null"}
	if sock != "" {
		targets["socket"] = sock
	}
	for name, target := range targets {
		t.Run(name, func(t *testing.T) {
			_, err := writeCompDBGuarded(t, target, pipe)
			if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
				t.Errorf("WriteCompDB(%s) = %v, want the error that it is not a regular file", target, err)
			}
			if fi, err := os.Lstat(target); err != nil || fi.Mode().IsRegular() {
				t.Errorf("%s was replaced by a file", target)
			}
		})
	}
	t.Run("link to a named pipe", func(t *testing.T) {
		link := filepath.Join(dir, "link.json")
		if err := os.Symlink(pipe, link); err != nil {
			t.Skip(err)
		}
		path, err := writeCompDBGuarded(t, link, pipe)
		if err != nil || path != link {
			t.Fatalf("WriteCompDB(link to a pipe) = %q, %v", path, err)
		}
		if fi, err := os.Lstat(link); err != nil || !fi.Mode().IsRegular() {
			t.Errorf("the link was not replaced by the database")
		}
		if fi, err := os.Lstat(pipe); err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
			t.Errorf("the pipe behind the link was changed")
		}
	})
}

// The name of a folder: an existing folder, or a name with a trailing slash (made when it is
// not there, like go build -o bin/); a name without the slash that is not there is a file; a link
// to a folder is not written through unless the slash says that the name is a folder.
func TestWriteCompDBFolderNames(t *testing.T) {
	root := t.TempDir()
	db := "compile_commands.json"
	t.Run("new folder with a slash", func(t *testing.T) {
		name := filepath.Join(root, "newdir") + "/"
		path, err := WriteCompDB(testPlan(), name)
		want := filepath.Join(root, "newdir", db)
		if err != nil || path != want || !exists(want) {
			t.Errorf("WriteCompDB(%s) = %q, %v; want %s", name, path, err, want)
		}
	})
	t.Run("new name without a slash is a file", func(t *testing.T) {
		name := filepath.Join(root, "newfile")
		path, err := WriteCompDB(testPlan(), name)
		if fi, serr := os.Stat(name); err != nil || path != name || serr != nil || !fi.Mode().IsRegular() {
			t.Errorf("WriteCompDB(%s) = %q, %v; want the file", name, path, err)
		}
	})
	t.Run("existing folder", func(t *testing.T) {
		name := filepath.Join(root, "folder")
		if err := os.Mkdir(name, 0o755); err != nil {
			t.Fatal(err)
		}
		path, err := WriteCompDB(testPlan(), name)
		if err != nil || path != filepath.Join(name, db) {
			t.Errorf("WriteCompDB(folder) = %q, %v", path, err)
		}
	})
	t.Run("file with a slash", func(t *testing.T) {
		name := filepath.Join(root, "afile")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := WriteCompDB(testPlan(), name+"/")
		if err == nil || !strings.Contains(err.Error(), "not a folder") {
			t.Errorf("WriteCompDB(file/) = %v, want an error that it is not a folder", err)
		}
		if b, _ := os.ReadFile(name); string(b) != "x" {
			t.Errorf("the file was written")
		}
	})
	t.Run("link to a folder", func(t *testing.T) {
		folder := filepath.Join(root, "behind")
		link := filepath.Join(root, "folderlink")
		if err := os.Mkdir(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(folder, link); err != nil {
			t.Skip(err)
		}
		_, err := WriteCompDB(testPlan(), link)
		if err == nil || !strings.Contains(err.Error(), "symbolic link to a folder") {
			t.Errorf("WriteCompDB(link to a folder) = %v, want the error about the link", err)
		}
		if exists(filepath.Join(folder, db)) {
			t.Errorf("the database was written behind the link")
		}
		if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("the link was replaced")
		}
		path, err := WriteCompDB(testPlan(), link+"/")
		if err != nil || path != filepath.Join(link, db) || !exists(filepath.Join(folder, db)) {
			t.Errorf("WriteCompDB(link/) = %q, %v; want the file in the folder", path, err)
		}
	})
}

// The automatic write names a file, never a folder to put one in: a folder, a link or a pipe
// with the name is left alone and reported, and a link to a folder is not resolved.
func TestWriteCompDBAutoLooksAtTheName(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	folder := filepath.Join(root, "folder")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(folder, link); err != nil {
		t.Skip(err)
	}
	for _, name := range []string{link, folder} {
		_, err := WriteCompDBOwned(testPlan(), name, cache, true)
		var foreign *ForeignCompDBError
		if !errors.As(err, &foreign) {
			t.Errorf("WriteCompDBOwned(%s, auto) = %v, want a ForeignCompDBError", name, err)
		}
		if exists(filepath.Join(folder, "compile_commands.json")) {
			t.Errorf("WriteCompDBOwned(%s, auto) wrote into the folder", name)
		}
	}
}
