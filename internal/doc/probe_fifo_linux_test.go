package doc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// TestFindHeaderNamedPipe: a system header that the include path (CPLUS_INCLUDE_PATH here) finds
// as a named pipe stops the compiler of FindHeader (build.WatchPipes) with an error that names the
// pipe, instead of a wait for a writer.
func TestFindHeaderNamedPipe(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "gocpp_fifo_header.h")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { // a stuck open goes on: nothing outlives the test
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	t.Setenv("CPLUS_INCLUDE_PATH", dir)
	t.Setenv("CCACHE_DISABLE", "1")
	p := &compilerProbe{goos: runtime.GOOS, goarch: runtime.GOARCH, dir: t.TempDir()}
	if _, err := p.cxx(context.Background()); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := p.FindHeader(ctx, "gocpp_fifo_header.h")
		done <- err
	}()
	var err error
	select {
	case err = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("FindHeader still runs after two minutes")
	}
	var pe *build.PipeError
	if !errors.As(err, &pe) {
		t.Fatalf("FindHeader = %v, want a *build.PipeError", err)
	}
}
