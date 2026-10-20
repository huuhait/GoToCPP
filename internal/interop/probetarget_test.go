package interop

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The probes of a program for another target run the compiler of that target with its flags
// (Phase 14 step 14.6, ADR-116): the argument lists, recorded by a fake compiler, and the keys of
// the probe cache.

// probeArgs runs the kind probe with the fake compiler cxx and returns the arguments it was given
// (the probe itself fails: the fake prints nothing).
func probeArgs(t *testing.T, cxx build.CXX, flags []string) []string {
	t.Helper()
	_, _ = newCache().probe(context.Background(), cxx, t.TempDir(), flags, []string{"<windows.h>"}, []string{"HANDLE"})
	data, err := os.ReadFile(cxx.Path + ".args")
	if err != nil {
		t.Fatalf("the fake compiler did not run: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

const recordArgs = `for a in "$@"; do printf '%s\n' "$a"; done > "$0.args"`

func TestProbeArgsForWindowsTarget(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no MinGW-w64 g++: the flags are the target's own
	win32 := []string{"-DWIN32_LEAN_AND_MEAN", "-DNOMINMAX", "-DUNICODE", "-D_UNICODE"}

	cxx := fakeCompiler(t, build.Clang, recordArgs)
	cxx.GOOS, cxx.GOARCH = "windows", "amd64"
	args := probeArgs(t, cxx, win32)
	for _, want := range []string{
		"--target=x86_64-w64-mingw32", "-D_WIN32_WINNT=0x0A00",
		"-DWIN32_LEAN_AND_MEAN", "-DNOMINMAX", "-DUNICODE", "-D_UNICODE",
	} {
		if !slices.Contains(args, want) {
			t.Errorf("the probe for windows lacks %s:\n%s", want, strings.Join(args, "\n"))
		}
	}
	// The package's flags come after the target's, so that they can override it.
	if slices.Index(args, "-D_WIN32_WINNT=0x0A00") > slices.Index(args, "-DNOMINMAX") {
		t.Errorf("the target's flags come after the package's:\n%s", strings.Join(args, "\n"))
	}

	// This machine's target: nothing of it.
	host := fakeCompiler(t, build.Clang, recordArgs)
	for _, a := range probeArgs(t, host, nil) {
		if strings.HasPrefix(a, "--target") || strings.Contains(a, "_WIN32_WINNT") {
			t.Errorf("the probe for this machine has %s", a)
		}
	}

	// g++ of MinGW-w64: no --target (it is the target's own), the API level only.
	gcc := fakeCompiler(t, build.GCC, recordArgs)
	gcc.GOOS, gcc.GOARCH = "windows", "amd64"
	args = probeArgs(t, gcc, win32)
	hasTarget := slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "--target") })
	if hasTarget || !slices.Contains(args, "-D_WIN32_WINNT=0x0A00") {
		t.Errorf("the probe of g++ for windows:\n%s", strings.Join(args, "\n"))
	}
}

// With the headers of a MinGW-w64 g++ at hand, a host's clang for windows is pointed at them.
func TestProbeArgsMingwHeaders(t *testing.T) {
	dir := t.TempDir()
	list := "#include <...> search starts here:\n /mingw/include/c++/15\n" +
		" /mingw/lib/gcc/x86_64-w64-mingw32/15/include\n /mingw/x86_64-w64-mingw32/include\n" +
		"End of search list.\n"
	script := "#!/bin/sh\nprintf '%s' '" + list + "' >&2\n"
	if err := os.WriteFile(filepath.Join(dir, "x86_64-w64-mingw32-g++"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	cxx := fakeCompiler(t, build.Clang, recordArgs)
	cxx.GOOS, cxx.GOARCH = "windows", "amd64"
	args := probeArgs(t, cxx, nil)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-nostdlibinc", "-nostdinc++",
		"-isystem /mingw/include/c++/15", "-isystem /mingw/x86_64-w64-mingw32/include",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the probe lacks %q:\n%s", want, strings.Join(args, "\n"))
		}
	}
	if strings.Contains(joined, "lib/gcc") {
		t.Errorf("the probe names GCC's own headers, which clang replaces:\n%s", strings.Join(args, "\n"))
	}
}

func TestProbeKeyHasTheTarget(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	base := build.CXX{Path: "/bin/clang++", Family: build.Clang, Version: "99"}
	win := base
	win.GOOS, win.GOARCH = "windows", "amd64"
	arm := base
	arm.GOOS, arm.GOARCH = "linux", "arm64"
	keys := map[string]string{}
	for name, c := range map[string]build.CXX{"host": base, "windows": win, "linux/arm64": arm} {
		k := setKey(c, "/d", []string{"-DX"}, []string{"<h>"})
		if other, ok := keys[k]; ok {
			t.Errorf("the keys of %s and %s are the same", name, other)
		}
		keys[k] = name
	}
	// The same clang with other target flags (the MinGW-w64 headers it is pointed at) is another key,
	// also when the number of flags is the same: the contents are in the key, not only their count.
	before := setKey(win, "/d", nil, nil) // no MinGW-w64 g++ in PATH
	fakeGXX := func(folder string) {
		dir := t.TempDir()
		script := "#!/bin/sh\nprintf '%s' '#include <...> search starts here:\n " + folder + "\nEnd of search list.\n' >&2\n"
		if err := os.WriteFile(filepath.Join(dir, "x86_64-w64-mingw32-g++"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
	}
	fakeGXX("/mingw-a/include")
	keyA := setKey(win, "/d", nil, nil)
	if len(build.ProbeTargetFlags(win)) == 0 {
		t.Fatal("the fake g++ gave no flags: the test shows nothing")
	}
	fakeGXX("/mingw-b/include")
	keyB := setKey(win, "/d", nil, nil)
	if keyA == keyB {
		t.Error("the key does not change with the folders of the target's headers (same number of flags)")
	}
	if keyA == before || keyB == before {
		t.Error("the key does not change with the headers of the target")
	}
}
