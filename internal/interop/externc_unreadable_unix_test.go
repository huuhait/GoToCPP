//go:build unix

package interop

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/load"
)

// The C header check of a header that can't be read or isn't a file: no error
// of its own (the probes report the header, so an import has one error), and,
// where the compiler would wait for ever, no compiler run.

// checkWithoutHang runs the C header check of include and fails the test
// instead of hanging when it waits for a writer of a named pipe. The cleanup
// opens the pipe, which lets a stuck open go on.
func checkWithoutHang(t *testing.T, c *cache, cc build.CXX, dir, include string, pipe string) (CHeader, error) {
	t.Helper()
	if pipe != "" {
		t.Cleanup(func() {
			if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
				f.Close()
			}
		})
	}
	type answer struct {
		res CHeader
		err error
	}
	done := make(chan answer, 1)
	go func() {
		res, err := c.checkCHeader(context.Background(), cc, dir, nil, include)
		done <- answer{res, err}
	}()
	select {
	case got := <-done:
		return got.res, got.err
	case <-time.After(time.Minute):
		t.Fatalf("the check of %s waits for a writer of a named pipe (no result after a minute)", include)
		return CHeader{}, nil
	}
}

func TestCheckCHeaderNamedPipe(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		pipe := filepath.Join(dir, "pipe.h")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		c := newCache()
		got, err := checkWithoutHang(t, c, cc, dir, `"./pipe.h"`, pipe)
		if err != nil || got != (CHeader{}) {
			t.Errorf("CheckCHeader(pipe) = %+v, %v; want no answer and no error", got, err)
		}
		if runs := c.cRuns.Load(); runs != 0 {
			t.Errorf("%d compiler runs for a named pipe, want none", runs)
		}
	})
}

func TestCheckCHeaderLinkToNamedPipe(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		pipe := filepath.Join(t.TempDir(), "pipe")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		if err := os.Symlink(pipe, filepath.Join(dir, "link.h")); err != nil {
			t.Skip(err)
		}
		c := newCache()
		got, err := checkWithoutHang(t, c, cc, dir, `"./link.h"`, pipe)
		if err != nil || got != (CHeader{}) || c.cRuns.Load() != 0 {
			t.Errorf("CheckCHeader(link to pipe) = %+v, %v, %d runs; want no answer, no error, no run",
				got, err, c.cRuns.Load())
		}
	})
}

func TestCheckCHeaderFolderNamedLikeHeader(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "dir.h"), 0o755); err != nil {
			t.Fatal(err)
		}
		c := newCache()
		got, err := checkWithoutHang(t, c, cc, dir, `"./dir.h"`, "")
		if err != nil || got != (CHeader{}) || c.cRuns.Load() != 0 {
			t.Errorf("CheckCHeader(folder) = %+v, %v, %d runs; want no answer, no error, no run",
				got, err, c.cRuns.Load())
		}
	})
}

// A header that is a named pipe now and a file later gets its answer then: the
// first one is not kept.
func TestCheckCHeaderNamedPipeAnswerNotKept(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		pipe := filepath.Join(dir, "later.h")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		c := newCache()
		if _, err := checkWithoutHang(t, c, cc, dir, `"./later.h"`, pipe); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(pipe); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, dir, map[string]string{"later.h": "int later(void);\n"})
		got, err := checkWithoutHang(t, c, cc, dir, `"./later.h"`, "")
		if err != nil || !got.C {
			t.Errorf("CheckCHeader(file that was a pipe) = %+v, %v; want a C header", got, err)
		}
	})
}

// The probes say "Permission denied" at the import (check's test); the C
// check has no error of its own for a header that can't be read, and it still
// asks the compiler (only a header that is no file is skipped).
func TestCheckCHeaderUnreadable(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"noperm.h": "int noperm(void);\n"})
		path := filepath.Join(dir, "noperm.h")
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		if f, err := os.Open(path); err == nil {
			f.Close()
			t.Skip("the file can be read without permission (root?)")
		}
		c := newCache()
		got, err := c.checkCHeader(context.Background(), cc, dir, nil, `"./noperm.h"`)
		if err != nil || got != (CHeader{}) {
			t.Errorf("CheckCHeader(noperm.h) = %+v, %v; want no answer and no error", got, err)
		}
		if runs := c.cRuns.Load(); runs != 1 {
			t.Errorf("%d compiler runs for a header without permission, want 1", runs)
		}
	})
}

// The state of a file that is a named pipe is an error that says so, not an
// empty file that exists, and it does not wait for a writer.
func TestStateOfNamedPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe.h")
	if err := syscall.Mkfifo(pipe, 0o644); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { // lets a stuck open go on: no goroutine outlives the test
		if f, err := os.OpenFile(pipe, os.O_RDWR, 0); err == nil {
			f.Close()
		}
	})
	type answer struct {
		state fileState
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		state, _, err := stateOf(pipe)
		done <- answer{state, err}
	}()
	select {
	case got := <-done:
		if !load.IsNotRegular(got.err) || got.state.exists {
			t.Errorf("stateOf(pipe) = %+v, %v; want an error that it is not a regular file", got.state, got.err)
		}
	case <-time.After(time.Minute):
		t.Fatal("stateOf waits for a writer of the named pipe (no result after a minute)")
	}
}

// A header that was a file and is a named pipe now: the cache looks at it again
// (it had its content), and must not wait for a writer.
func TestCheckCHeaderFileSwappedForNamedPipe(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"swap.h": "int swap(void);\n"})
		c := newCache()
		got, err := checkWithoutHang(t, c, cc, dir, `"./swap.h"`, "")
		if err != nil || !got.C {
			t.Fatalf("CheckCHeader(file) = %+v, %v; want a C header", got, err)
		}
		path := filepath.Join(dir, "swap.h")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(path, 0o644); err != nil {
			t.Skip(err)
		}
		got, err = checkWithoutHang(t, c, cc, dir, `"./swap.h"`, path)
		if err != nil || got != (CHeader{}) {
			t.Errorf("CheckCHeader(pipe that was a file) = %+v, %v; want no answer and no error", got, err)
		}
	})
}
