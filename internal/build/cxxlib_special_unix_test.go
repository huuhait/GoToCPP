//go:build unix

package build

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestLibSourcesSpecialFiles: a named pipe as a source would block the build
// forever when it is read; it is an error at the recipe line, also when a
// pattern matches it. (The listing itself must not block either.)
func TestLibSourcesSpecialFiles(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "ok.c")
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.c"), 0o644); err != nil {
		t.Skip("no named pipes:", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, line := range []string{"src pipe.c", "src *.c", "src p*.c"} {
			got := errMsgs(libOf(t, dir, "\t"+line))
			if len(got) != 1 || !strings.Contains(got[0], "not a regular file (it is a named pipe)") {
				t.Errorf("%s: %q", line, got)
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the listing of a source that is a named pipe blocks")
	}
}
