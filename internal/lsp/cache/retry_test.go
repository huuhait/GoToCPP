package cache

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A read that fails because the process has no file descriptors for a
// moment is tried again; any other error is the answer at once.
func TestRetryRead(t *testing.T) {
	calls := 0
	v, err := retryRead(func() (int, error) {
		calls++
		if calls < 3 {
			return 0, &fs.PathError{Op: "open", Path: "x", Err: syscall.EMFILE}
		}
		return 7, nil
	})
	if v != 7 || err != nil || calls != 3 {
		t.Errorf("got %d, %v after %d calls, want 7 after 3", v, err, calls)
	}
	calls = 0
	_, err = retryRead(func() (int, error) { calls++; return 0, fs.ErrPermission })
	if !errors.Is(err, fs.ErrPermission) || calls != 1 {
		t.Errorf("a permission error: %v after %d calls, want it at once", err, calls)
	}
	calls = 0
	_, err = retryRead(func() (int, error) { calls++; return 0, syscall.EMFILE })
	if !errors.Is(err, syscall.EMFILE) || calls != 4 {
		t.Errorf("EMFILE forever: %v after %d calls, want 4 and the error", err, calls)
	}
}

// A folder that can't be read, with an open document in it that makes its
// listing not empty, is a failed listing all the same: the next snapshot
// reads the folder again (and so sees it readable when it is).
func TestFailedListingWithOverlay(t *testing.T) {
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"lib/a.gp":   "package lib\n",
		"other/o.gp": "package other\n",
	})
	lib := filepath.Join(w.root, "lib")
	if err := os.Chmod(lib, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(lib, 0o755)
	if _, err := os.ReadDir(lib); err == nil {
		t.Skip("the folder is readable with mode 0 (root)")
	}
	w.open("lib/b.gp", "package lib\n")
	dir := func() *dirHandle {
		snap, release := w.s.Snapshot()
		defer release()
		st := snap.State().(*state)
		d, err := st.dir(lib)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	d := dir()
	if !d.failed || len(d.names) != 1 || d.names[0] != "b.gp" {
		t.Fatalf("the listing: failed %v, names %v; want failed, [b.gp]", d.failed, d.names)
	}
	if err := os.Chmod(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	w.open("other/o.gp", "package other\n") // a new snapshot, no change in lib
	d = dir()
	if d.failed || len(d.names) != 2 {
		t.Errorf("the next snapshot kept the failed listing: failed %v, names %v; want a-and-b read again", d.failed, d.names)
	}
}
