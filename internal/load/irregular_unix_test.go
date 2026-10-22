//go:build unix

package load

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadRegularAsksTheOpenFile: the kind is asked of the file that was opened, not of its name
// afterwards, so a name that is swapped for a named pipe between the open and the look still
// reads the file that was opened. (A look at the name would see the pipe and refuse the file.)
func TestReadRegularAsksTheOpenFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "a.gp")
	if err := os.WriteFile(name, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterOpenHook = func(opened string) {
		if err := os.Remove(opened); err != nil {
			t.Error(err)
		}
		if err := mkfifo(opened); err != nil {
			t.Error(err)
		}
	}
	defer func() { afterOpenHook = nil }()
	got, err := ReadRegular(name)
	if err != nil || string(got) != "text" {
		t.Errorf("ReadRegular = %q, %v; want the text of the file that was opened", got, err)
	}
}

// TestReadRegularSocket: the open of a unix socket fails with ENXIO ("no such device or
// address"); ReadRegular says that it is a socket.
func TestReadRegularSocket(t *testing.T) {
	short, err := os.MkdirTemp("", "gs")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(short)
	sock := filepath.Join(short, "s")
	lis, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip(err)
	}
	defer lis.Close()
	_, err = ReadRegular(sock)
	if err == nil || !strings.Contains(err.Error(), "not a regular file: it is a socket") {
		t.Errorf("ReadRegular(socket) = %v, want that it is a socket", err)
	}
}
