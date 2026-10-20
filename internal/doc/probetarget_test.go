package doc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// fakeClang writes a clang++ that records its arguments (one line per run in <path>.log) and finds
// no header: the probes of gocpp doc then answer "no file", and the arguments are what is checked.
func fakeClang(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	script := `#!/bin/sh
for a in "$@"; do
	case "$a" in
	--version) echo "clang version 99.0.0"; exit 0;;
	-dM) echo "#define __clang__ 1"; echo "#define __clang_major__ 99"; echo "#define __GNUC__ 4"; exit 0;;
	esac
done
echo "$*" >> "$0.log"
exit 1
`
	path := filepath.Join(dir, "clang++")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// lastRun returns the arguments of the last run of the fake compiler at path.
func lastRun(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path + ".log")
	if err != nil {
		t.Fatalf("the fake compiler did not run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	return strings.Fields(lines[len(lines)-1])
}

// gocpp doc of a header for windows asks the compiler of the target with the target's flags and the
// Win32 defaults (the header found is the one a package for windows includes); for the host's own
// target it passes neither.
func TestDocProbeWindowsTarget(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no MinGW-w64 g++: the target's own flags only
	t.Setenv("GOCPP_CFLAGS", "")
	t.Setenv("GOCPP_CXXFLAGS", "")
	clang := fakeClang(t, t.TempDir())
	t.Setenv("GOCPP_CXX", clang)
	ctx := context.Background()

	win := &compilerProbe{goos: "windows", goarch: "amd64", dir: t.TempDir()}
	if _, err := win.FindHeader(ctx, "windows.h"); err != nil {
		t.Fatal(err)
	}
	args := lastRun(t, clang)
	for _, want := range []string{
		"--target=x86_64-w64-mingw32", "-D_WIN32_WINNT=0x0A00",
		"-DWIN32_LEAN_AND_MEAN", "-DNOMINMAX", "-DUNICODE", "-D_UNICODE", "-E", "-M",
	} {
		if !slices.Contains(args, want) {
			t.Errorf("FindHeader for windows: the compiler ran with %q, lacking %s", args, want)
		}
	}

	// The kind probe: the defaults are part of its flags.
	_, _ = win.Kind(ctx, []string{"<windows.h>"}, "HANDLE")
	args = lastRun(t, clang)
	for _, want := range []string{"--target=x86_64-w64-mingw32", "-DNOMINMAX", "-DUNICODE"} {
		if !slices.Contains(args, want) {
			t.Errorf("Kind for windows: the compiler ran with %q, lacking %s", args, want)
		}
	}

	if runtime.GOOS == "windows" {
		return
	}
	host := &compilerProbe{goos: runtime.GOOS, goarch: runtime.GOARCH, dir: t.TempDir()}
	if _, err := host.FindHeader(ctx, "vector"); err != nil {
		t.Fatal(err)
	}
	for _, a := range lastRun(t, clang) {
		if strings.HasPrefix(a, "--target") || strings.Contains(a, "WIN32") || a == "-DNOMINMAX" || a == "-DUNICODE" {
			t.Errorf("FindHeader for this machine has %s", a)
		}
	}
}
