//go:build unix

package cache

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/load"
)

// TestNamedPipeSourceDoesNotBlock: a named pipe with the name of a source in a package is an
// error of the read, not a read that waits for a writer for ever (the snapshot's reads, and the
// check of the package, which hover and workspace/symbol wait for).
func TestNamedPipeSourceDoesNotBlock(t *testing.T) {
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "package main\n\nfunc main() {}\n",
	})
	pipe := filepath.Join(w.root, "pipe.gp")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { // a stuck read goes on, so that no goroutine outlives the test
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		snap, release := w.s.Snapshot()
		defer release()
		if _, err := snap.ReadFile(pipe); !load.IsNotRegular(err) {
			t.Errorf("ReadFile(pipe.gp) = %v, want the error of a file that is not regular", err)
		}
		if _, err := snap.Package(context.Background(), w.root); err != nil && !load.IsNotRegular(err) {
			t.Errorf("Package(.) = %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Minute):
		t.Fatal("the snapshot waits for a writer of the named pipe pipe.gp (no result after a minute)")
	}
}
