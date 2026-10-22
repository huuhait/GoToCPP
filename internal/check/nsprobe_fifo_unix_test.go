//go:build unix

package check

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// TestProbeHeaderIsNamedPipe checks a named pipe with the name of a header:
// one error that says what it is, and no wait of a compiler for a writer.
func TestProbeHeaderIsNamedPipe(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		prog, root := loadTree(t, map[string]string{
			"main.gp": "package main\n\nimport \"./pipe.h\"\n\nvar v = std.vector[int]{}\n",
		})
		pipe := filepath.Join(root, "pipe.h")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { // lets a stuck open go on: no goroutine outlives the test
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		})
		done := make(chan string, 1)
		go func() { done <- errorText(t, prog, Check(prog, probeConfig(cxx, LastPhase)), root, true) }()
		select {
		case got := <-done:
			want := "# example.com/m\n" +
				"./main.gp:3:8: cannot check the header imports: kind probe: open " + slashed(pipe) +
				": not a regular file: it is a named pipe\n"
			if got != want {
				t.Errorf("errors:\n%s\nwant:\n%s", got, want)
			}
		case <-time.After(time.Minute):
			t.Fatal("Check waits for a writer of the named pipe (no result after a minute)")
		}
	})
}
