package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFake writes a compiler named name into dir: --version and -dM answer as clang (or as g++
// when clang is false).
func writeFake(t *testing.T, dir, name string, clang bool) string {
	t.Helper()
	macros := `echo "#define __GNUC__ 99"; echo '#define __VERSION__ "99.1.0"'`
	version := "g++ (GCC) 99.1.0"
	if clang {
		macros = `echo "#define __GNUC__ 4"; echo "#define __clang__ 1"; ` +
			`echo "#define __clang_major__ 99"; echo '#define __VERSION__ "Clang 99.0.0"'`
		version = "clang version 99.0.0"
	}
	script := "#!/bin/sh\nfor a in \"$@\"; do\n\tcase \"$a\" in\n" +
		"\t--version) echo '" + version + "'; exit 0;;\n" +
		"\t-dM) " + macros + "; exit 0;;\n\tesac\ndone\nexit 3\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindClangFor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	ctx := context.Background()
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("GOCPP_CLANG", "")

	t.Run("this machine", func(t *testing.T) {
		dir := t.TempDir()
		writeFake(t, dir, "clang++", true)
		t.Setenv("PATH", dir)
		cxx, err := FindClangFor(ctx, runtime.GOOS, runtime.GOARCH)
		if runtime.GOOS == "windows" {
			return
		}
		if err != nil || cxx.GOOS != "" {
			t.Errorf("FindClangFor(host) = %q (GOOS %q), %v: want clang++ with no target", cxx.Path, cxx.GOOS, err)
		}
		if cxx, err = FindClangFor(ctx, "", ""); err != nil || cxx.GOOS != "" {
			t.Errorf("FindClangFor(empty) = %q (GOOS %q), %v", cxx.Path, cxx.GOOS, err)
		}
	})

	t.Run("windows prefers the clang of MinGW-w64", func(t *testing.T) {
		dir := t.TempDir()
		writeFake(t, dir, "clang++", true)
		mingw := writeFake(t, dir, "x86_64-w64-mingw32-clang++", true)
		writeFake(t, dir, "x86_64-w64-mingw32-g++", false)
		t.Setenv("PATH", dir)
		cxx, err := FindClangFor(ctx, "windows", "amd64")
		if err != nil || cxx.Path != mingw || cxx.GOOS != "windows" || cxx.GOARCH != "amd64" {
			t.Errorf("FindClangFor(windows) = %q (%s/%s), %v; want %s for windows/amd64",
				cxx.Path, cxx.GOOS, cxx.GOARCH, err, mingw)
		}
	})

	t.Run("windows with the clang of the host", func(t *testing.T) {
		dir := t.TempDir()
		host := writeFake(t, dir, "clang++", true)
		t.Setenv("PATH", dir)
		cxx, err := FindClangFor(ctx, "windows", "amd64")
		if err != nil || cxx.Path != host || cxx.GOOS != "windows" {
			t.Errorf("FindClangFor(windows) = %q (%s), %v; want the host's clang++ as a compiler for windows",
				cxx.Path, cxx.GOOS, err)
		}
	})

	t.Run("GOCPP_CLANG decides", func(t *testing.T) {
		dir := t.TempDir()
		writeFake(t, dir, "clang++", true)
		mine := writeFake(t, t.TempDir(), "my-clang", true)
		t.Setenv("PATH", dir)
		t.Setenv("GOCPP_CLANG", mine)
		cxx, err := FindClangFor(ctx, "windows", "amd64")
		if err != nil || cxx.Path != mine || cxx.GOOS != "windows" {
			t.Errorf("FindClangFor with GOCPP_CLANG = %q (%s), %v", cxx.Path, cxx.GOOS, err)
		}
		// A GOCPP_CLANG that is not clang is an error, not a fall back to another compiler.
		t.Setenv("GOCPP_CLANG", writeFake(t, t.TempDir(), "my-gcc", false))
		if _, err := FindClangFor(ctx, "windows", "amd64"); !errors.Is(err, ErrNoClang) {
			t.Errorf("FindClangFor with a GOCPP_CLANG that is g++: %v, want ErrNoClang", err)
		}
	})

	t.Run("windows without clang", func(t *testing.T) {
		dir := t.TempDir()
		writeFake(t, dir, "g++", false)
		writeFake(t, dir, "x86_64-w64-mingw32-g++", false)
		t.Setenv("PATH", dir)
		if _, err := FindClangFor(ctx, "windows", "amd64"); !errors.Is(err, ErrNoClang) {
			t.Errorf("FindClangFor without clang: %v, want ErrNoClang", err)
		}
	})

	t.Run("another target gets the clang of this machine", func(t *testing.T) {
		dir := t.TempDir()
		host := writeFake(t, dir, "clang++", true)
		t.Setenv("PATH", dir)
		cxx, err := FindClangFor(ctx, "plan9", "riscv64")
		if err != nil || cxx.Path != host || cxx.GOOS != "" {
			t.Errorf("FindClangFor(plan9) = %q (GOOS %q), %v; want the host's clang++ with no target", cxx.Path, cxx.GOOS, err)
		}
	})
}

func TestProbesTarget(t *testing.T) {
	for _, c := range []struct {
		goos, goarch string
		want         bool
	}{
		{"", "", true},
		{runtime.GOOS, runtime.GOARCH, true},
		{"windows", "amd64", true},
		{"plan9", "riscv64", false},
		{"windows", "386", runtime.GOOS == "windows" && runtime.GOARCH == "386"},
	} {
		if got := ProbesTarget(c.goos, c.goarch); got != c.want {
			t.Errorf("ProbesTarget(%q, %q) = %v, want %v", c.goos, c.goarch, got, c.want)
		}
	}
}
