//go:build unix

package build

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMoveFileDevNull(t *testing.T) {
	// `gocpp build -o /dev/null` writes the program into /dev/null, as
	// `go build -o /dev/null` does, instead of a temporary file in /dev.
	from := filepath.Join(t.TempDir(), "prog")
	if err := os.WriteFile(from, []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(from, os.DevNull); err != nil {
		t.Fatalf("MoveFile(%q, %q): %v", from, os.DevNull, err)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Errorf("%s still exists after the move (%v)", from, err)
	}
	fi, err := os.Stat(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeDevice == 0 {
		t.Errorf("%s is no longer a device: %v", os.DevNull, fi.Mode())
	}
}

func TestMoveFileNamedPipe(t *testing.T) {
	// A named pipe gets the program written into it and stays a pipe.
	dir := t.TempDir()
	from := filepath.Join(dir, "prog")
	if err := os.WriteFile(from, []byte("program"), 0o755); err != nil {
		t.Fatal(err)
	}
	pipe := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	got := make(chan []byte, 1)
	go func() {
		data, _ := os.ReadFile(pipe)
		got <- data
	}()
	if err := MoveFile(from, pipe); err != nil {
		t.Fatalf("MoveFile(%q, %q): %v", from, pipe, err)
	}
	select {
	case data := <-got:
		if string(data) != "program" {
			t.Errorf("read %q from the pipe, want %q", data, "program")
		}
	case <-time.After(10 * time.Second):
		t.Error("nothing was written into the pipe")
	}
	fi, err := os.Lstat(pipe)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("%s is no longer a named pipe: %v", pipe, fi.Mode())
	}
}
