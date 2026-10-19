package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// forgetDetected empties the in-process answers of the compiler detection, as a new run of gocpp
// would find them.
func forgetDetected(t *testing.T) {
	t.Helper()
	detected.Lock()
	detected.m = make(map[string]detection)
	detected.Unlock()
}

// The answer of the compiler detection is kept on disk for the program file that gave it: a new
// process does not run the compiler again, and a program file that was replaced is detected again.
func TestDetectCache(t *testing.T) {
	cache := t.TempDir()
	SetDetectCache(cache)
	t.Cleanup(func() { SetDetectCache("") })
	t.Setenv("GOCPP_NOOBJCACHE", "")
	dir := t.TempDir()
	prog := fakeCXX(t, dir, "g++", "g++ (fake) 99.1.0", gccMacros...)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(prog, old, old)
	ctx := context.Background()

	first, err := LookCXX(ctx, prog)
	if err != nil || first.Family != GCC || first.Version != "g++ (fake) 99.1.0" || first.Major != 99 {
		t.Fatalf("LookCXX = %+v, %v", first, err)
	}
	if n := runsOf(t, prog); n != 2 {
		t.Fatalf("the compiler ran %d times, want 2 (--version and -dM)", n)
	}
	if files, _ := filepath.Glob(filepath.Join(cache, "cxxid", "*.json")); len(files) != 1 {
		t.Fatalf("the cache folder has %v, want one answer", files)
	}

	// A new process: the in-process answers are gone, the disk has it.
	forgetDetected(t)
	again, err := LookCXX(ctx, prog)
	if err != nil || again.Family != first.Family || again.Version != first.Version || again.Major != first.Major || again.stamp != first.stamp || again.Path != first.Path {
		t.Fatalf("LookCXX from the disk = %+v, %v; want %+v", again, err, first)
	}
	if n := runsOf(t, prog); n != 2 {
		t.Errorf("the compiler ran again (%d runs) although its program file did not change", n)
	}

	// The program file is another one with the same version line: detected again (its size changed).
	if err := os.WriteFile(prog, append(mustRead(t, prog), "# another file\n"...), 0o755); err != nil {
		t.Fatal(err)
	}
	forgetDetected(t)
	if _, err := LookCXX(ctx, prog); err != nil {
		t.Fatal(err)
	}
	if n := runsOf(t, prog); n != 4 {
		t.Errorf("after the program file changed the compiler ran %d times in all, want 4", n)
	}

	// A damaged answer is no answer.
	files, _ := filepath.Glob(filepath.Join(cache, "cxxid", "*.json"))
	for _, f := range files {
		os.WriteFile(f, []byte("{not json"), 0o644)
	}
	forgetDetected(t)
	if got, err := LookCXX(ctx, prog); err != nil || got.Family != GCC {
		t.Fatalf("LookCXX with a damaged answer = %+v, %v", got, err)
	}
	if n := runsOf(t, prog); n != 6 {
		t.Errorf("a damaged answer was used: %d runs in all, want 6", n)
	}

	// GOCPP_NOOBJCACHE turns it off, as it does for the object cache; and with no cache folder there is none.
	t.Setenv("GOCPP_NOOBJCACHE", "1")
	forgetDetected(t)
	if _, err := LookCXX(ctx, prog); err != nil {
		t.Fatal(err)
	}
	if n := runsOf(t, prog); n != 8 {
		t.Errorf("with GOCPP_NOOBJCACHE the compiler ran %d times in all, want 8", n)
	}
	t.Setenv("GOCPP_NOOBJCACHE", "")
	SetDetectCache("")
	forgetDetected(t)
	if _, err := LookCXX(ctx, prog); err != nil {
		t.Fatal(err)
	}
	if n := runsOf(t, prog); n != 10 {
		t.Errorf("without a cache folder the compiler ran %d times in all, want 10", n)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A program file that was replaced by one of the same size, with its old modification time, is
// detected again: the record on disk has the content hash too.
func TestDetectCacheSameSizeSameTime(t *testing.T) {
	cache := t.TempDir()
	SetDetectCache(cache)
	t.Cleanup(func() { SetDetectCache("") })
	t.Setenv("GOCPP_NOOBJCACHE", "")
	prog := fakeCXX(t, t.TempDir(), "g++", "g++ (fake) 99.1.0", gccMacros...)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(prog, old, old)
	ctx := context.Background()
	if c, err := LookCXX(ctx, prog); err != nil || c.Version != "g++ (fake) 99.1.0" {
		t.Fatalf("LookCXX = %+v, %v", c, err)
	}
	data := mustRead(t, prog)
	if err := os.WriteFile(prog, []byte(strings.ReplaceAll(string(data), "99.1.0", "99.2.0")), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(prog, old, old)
	forgetDetected(t)
	c, err := LookCXX(ctx, prog)
	if err != nil || c.Version != "g++ (fake) 99.2.0" {
		t.Errorf("the replaced compiler is detected as %+v, %v; want the new version", c, err)
	}
}
