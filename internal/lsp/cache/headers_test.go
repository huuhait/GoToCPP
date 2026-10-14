package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/load"
)

func TestParseIncludeDirs(t *testing.T) {
	out := `Using built-in specs.
#include "..." search starts here:
 /quoted/only
#include <...> search starts here:
 /usr/lib/gcc/x86_64-linux-gnu/15/../../../../include/c++/15
 /usr/include
 /System/Library/Frameworks (framework directory)
End of search list.
 /not/this
`
	got := parseIncludeDirs([]byte(out))
	want := []string{filepath.FromSlash("/usr/include/c++/15"), filepath.FromSlash("/usr/include")}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestIncludeDirsRetry: a compiler run that failed is kept for cxxRetry,
// then the compiler is asked again.
func TestIncludeDirsRetry(t *testing.T) {
	old := cxxRetry
	cxxRetry = 50 * time.Millisecond
	t.Cleanup(func() { cxxRetry = old })
	var mu sync.Mutex
	runs := 0
	cfg := testConfig()
	cfg.IncludeDirs = func(context.Context, build.CXX) ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		runs++
		if runs == 1 {
			return nil, errors.New("timed out")
		}
		return []string{"/usr/include"}, nil
	}
	p := NewPipeline(cfg)
	t.Cleanup(p.Close)
	cxx := build.CXX{Path: "/fake/c++", Family: build.GCC}
	ctx := context.Background()
	for range 2 {
		if _, err := p.includeDirs(ctx, cxx); err == nil {
			t.Fatal("no error before cxxRetry")
		}
	}
	if runs != 1 {
		t.Errorf("%d runs before cxxRetry, want 1", runs)
	}
	time.Sleep(cxxRetry)
	dirs, err := p.includeDirs(ctx, cxx)
	if err != nil || !slices.Equal(dirs, []string{"/usr/include"}) {
		t.Errorf("after cxxRetry: %q, %v", dirs, err)
	}
	if _, err := p.includeDirs(ctx, cxx); err != nil || runs != 2 {
		t.Errorf("%d runs (%v), want 2: the answer is kept", runs, err)
	}
}

// TestHeaderFileAbsolute: an absolute header path is the file itself, as
// for the compiler, not a name in the folder of the .gp file.
func TestHeaderFileAbsolute(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "x.h")
	if err := os.WriteFile(abs, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewPipeline(testConfig())
	t.Cleanup(p.Close)
	for _, inc := range []*load.Include{
		{Kind: load.LocalHeader, Path: filepath.ToSlash(abs)},
		{Kind: load.SystemHeader, Path: "<" + filepath.ToSlash(abs) + ">"},
	} {
		got, err := p.HeaderFile(context.Background(), dir, inc)
		if err != nil || got != abs {
			t.Errorf("%s: %q, %v; want %q", inc.Path, got, err, abs)
		}
	}
	got, err := p.HeaderFile(context.Background(), dir, &load.Include{Kind: load.LocalHeader, Path: filepath.ToSlash(filepath.Join(dir, "nosuch.h"))})
	if err != nil || got != "" {
		t.Errorf("a missing absolute header: %q, %v", got, err)
	}
}
