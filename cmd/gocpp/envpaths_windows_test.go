//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/winpath"
)

// A TEMP of 252 characters (CreateProcess takes a working folder of at most 247): TEMP is replaced by a folder that
// a compiler can be started in. This runs on the real PC (tests/windows_native_test.go).
func TestNormalizeLongTemp(t *testing.T) {
	long := t.TempDir()
	for len(long) < 252 {
		long = filepath.Join(long, strings.Repeat("t", 28))
	}
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Skipf("cannot make a path of %d characters here: %v", len(long), err)
	}
	t.Setenv("TEMP", long)
	t.Setenv("TMP", long)
	old := originalEnv
	originalEnv = map[string]string{}
	t.Cleanup(func() { originalEnv = old })
	changed := normalizeEnvPaths(osPathsEnv(winpath.ACPSafe, winpath.Short, winpath.PrivateFolder))
	if len(changed) != 2 {
		t.Fatalf("changed %v", changed)
	}
	if got := os.Getenv("TEMP"); len(got) > maxTempLen || got == long {
		t.Errorf("TEMP = %q (%d characters)", got, len(got))
	}
	if programEnv()[0] == "" || originalEnv["TEMP"] != long {
		t.Errorf("the original TEMP was not kept: %q", originalEnv["TEMP"])
	}
	// A folder to work in is made below the new TEMP.
	dir, err := os.MkdirTemp("", "gocpp-build")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(dir)
}
