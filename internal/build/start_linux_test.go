package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStartTextFileBusy(t *testing.T) {
	// A program that is still open for writing can't be executed
	// (ETXTBSY): Start tries again until it can.
	path := filepath.Join(t.TempDir(), "prog")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("#!/bin/sh\necho ok\n"); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(path).Run(); err == nil {
		t.Skip("this system runs a program that is open for writing")
	}
	time.AfterFunc(150*time.Millisecond, func() { f.Close() })
	var out []byte
	cmd, err := Start(context.Background(), func() *exec.Cmd {
		cmd := exec.Command(path)
		cmd.Stdout = &writer{&out}
		return cmd
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := cmd.Wait(); err != nil || string(out) != "ok\n" {
		t.Errorf("the program: %v, output %q", err, out)
	}
}

type writer struct{ b *[]byte }

func (w *writer) Write(p []byte) (int, error) {
	*w.b = append(*w.b, p...)
	return len(p), nil
}
