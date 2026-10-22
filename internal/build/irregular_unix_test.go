//go:build unix

package build

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// guardedPipe runs f and fails the test instead of hanging when it waits for a writer of the named
// pipe: no result for a minute is a hang. The cleanup opens the pipe once, so that a stuck read
// goes on and no goroutine outlives the test.
func guardedPipe(t *testing.T, pipe string, f func()) {
	t.Helper()
	t.Cleanup(func() {
		if file, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			file.Close()
		}
	})
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Minute):
		t.Fatalf("waits for a writer of the named pipe %s (no result after a minute)", pipe)
	}
}

// TestWriteFileIfChangedReplacesAPipe: a named pipe in the place of the file is replaced by the
// file, not read (a read waits for a writer for ever).
func TestWriteFileIfChangedReplacesAPipe(t *testing.T) {
	name := filepath.Join(t.TempDir(), "out.json")
	if err := syscall.Mkfifo(name, 0o644); err != nil {
		t.Skip(err)
	}
	var err error
	guardedPipe(t, name, func() { err = WriteFileIfChanged(name, []byte("data")) })
	got, rerr := os.ReadFile(name)
	if err != nil || rerr != nil || string(got) != "data" {
		t.Errorf("WriteFileIfChanged over a pipe = %v; file %q, %v; want the file with the data", err, got, rerr)
	}
}

// TestWriteFileIfChangedKeepsAFileWithTheSameBytes: a regular file with the bytes is not written.
func TestWriteFileIfChangedKeepsAFileWithTheSameBytes(t *testing.T) {
	name := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(name)
	if err := WriteFileIfChanged(name, []byte("data")); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(name)
	if !os.SameFile(before, after) || after.Mode().Perm() != 0o600 {
		t.Errorf("the file was written again: %v then %v", before.Mode(), after.Mode())
	}
}

// TestFSRecorderRefusesAPipe: the reads of gocpp build go through the recorder, so a named pipe
// with the name of a source or of the module file is an error there too, not a wait.
func TestFSRecorderRefusesAPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "a.gp")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	rec := NewFSRecorder()
	var err error
	guardedPipe(t, pipe, func() { _, err = rec.ReadFile(pipe) })
	if err == nil || !strings.Contains(err.Error(), "it is a named pipe") {
		t.Errorf("ReadFile(pipe) = %v, want an error about the named pipe", err)
	}
	guardedPipe(t, pipe, func() { _, err = rec.Open(pipe) })
	if err == nil || !strings.Contains(err.Error(), "it is a named pipe") {
		t.Errorf("Open(pipe) = %v, want an error about the named pipe", err)
	}
}
