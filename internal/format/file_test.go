package format

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// dirNames returns the names in the folder dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestWriteFile checks the safe write of gofmt -w: the file gets the new
// contents, a shorter or a longer one, and keeps its mode, and the backup
// file is gone at the end.
func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "x.gp")
	contents := []string{
		"package  p\n\n// A long comment, so that the formatted file is shorter.\nvar  x = 1\n",
		"package p\n",
		"package p\n\nvar x = 1 // longer again\n",
	}
	if err := os.WriteFile(name, []byte(contents[0]), 0o600); err != nil {
		t.Fatal(err)
	}
	// Not 0o600 and not the umask's default, so that a new file would not
	// get it by chance. (On Windows, only the read-only bit exists.)
	const perm = 0o750
	if err := os.Chmod(name, perm); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(contents); i++ {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteFile(name, []byte(contents[i-1]), []byte(contents[i]), info.Mode().Perm(), info.Size()); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if got := string(readFile(t, name)); got != contents[i] {
			t.Errorf("after WriteFile, the file holds %q, want %q", got, contents[i])
		}
		info, err = os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != perm {
			t.Errorf("after WriteFile, the mode is %v, want %v", info.Mode().Perm(), os.FileMode(perm))
		}
		if names := dirNames(t, dir); !slices.Equal(names, []string{"x.gp"}) {
			t.Errorf("after WriteFile, the folder holds %q, want only x.gp: the backup must be gone", names)
		}
	}
}

// TestWriteFileReadOnly checks a file that can't be written, like gofmt's
// TestPermissions: WriteFile fails, and the file, its time and its folder
// stay as they were.
func TestWriteFileReadOnly(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "x.gp")
	const src = "package  p\n"
	if err := os.WriteFile(name, []byte(src), 0o400); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(name, past, past); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	err = WriteFile(name, []byte(src), []byte("package p\n"), info.Mode().Perm(), info.Size())
	if err == nil {
		t.Fatal("WriteFile wrote a read-only file")
	}
	t.Log(err)
	if got := string(readFile(t, name)); got != src {
		t.Errorf("the file holds %q, want %q", got, src)
	}
	if info, err := os.Stat(name); err != nil {
		t.Fatal(err)
	} else if !info.ModTime().Equal(past) {
		t.Errorf("the file has the time %v, want %v", info.ModTime(), past)
	}
	if names := dirNames(t, dir); !slices.Equal(names, []string{"x.gp"}) {
		t.Errorf("the folder holds %q, want only x.gp: the backup must be gone", names)
	}
}

// TestReadFile checks ReadFile, also when the file is not the size that
// info says: then someone changed it while gocpp fmt read it.
func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "x.gp")
	const src = "package p\n"
	if err := os.WriteFile(name, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []os.FileInfo{info, nil} {
		got, err := ReadFile(name, i)
		if err != nil || string(got) != src {
			t.Errorf("ReadFile(info %v) = %q, %v; want %q", i != nil, got, err, src)
		}
	}
	for _, tc := range []struct{ now, err string }{
		{"package p\n\nvar x = 1\n", "size of " + name + " changed during reading (from 10 to >=11 bytes)"},
		{"package\n", "size of " + name + " changed during reading (from 10 to 8 bytes)"},
	} {
		if err := os.WriteFile(name, []byte(tc.now), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ReadFile(name, info)
		if err == nil || !strings.Contains(err.Error(), tc.err) {
			t.Errorf("ReadFile of a changed file: %v, want an error with %q", err, tc.err)
		}
	}
	if _, err := ReadFile(filepath.Join(dir, "none.gp"), nil); !os.IsNotExist(err) {
		t.Errorf("ReadFile of a missing file: %v, want a not-exist error", err)
	}
}
