package build_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// loadFor is loadPkg for another target.
func loadFor(t *testing.T, goos string, files map[string]string) (*load.Package, *load.Program) {
	t.Helper()
	dir := filepath.Clean(t.TempDir())
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog := load.Load(load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: goos, GOARCH: "amd64"}, ".")
	lp := prog.Roots[0]
	if lp.Errors.HasErrors() {
		t.Fatalf("load errors: %v", lp.Errors.Diagnostics())
	}
	return lp, prog
}

var win32All = []string{"-DWIN32_LEAN_AND_MEAN", "-DNOMINMAX", "-DUNICODE", "-D_UNICODE"}

const winMain = `package main

import "<windows.h>"

func main() { windows.GetTickCount64() }
`

// TestWin32Defaults checks the rules of step 14.4 (ADR-082): the defaults are for a package for windows
// with a header import or C/C++ files, they come first (the user's flags win), the directive turns them
// off, and a macro that the user's flags mention is left alone, in every kind of flag.
func TestWin32Defaults(t *testing.T) {
	t.Setenv("GOCPP_CFLAGS", "")
	t.Setenv("GOCPP_CXXFLAGS", "")
	read := func(t *testing.T, goos string, files map[string]string) build.PackageFlags {
		t.Helper()
		lp, prog := loadFor(t, goos, files)
		flags, errs, _ := build.ReadPackageFlags(t.Context(), prog.Fset, lp, nil)
		if len(errs) > 0 {
			t.Fatalf("errors: %v", errs)
		}
		return flags
	}
	t.Run("windows with a header", func(t *testing.T) {
		f := read(t, "windows", map[string]string{"main.gp": winMain})
		if !slices.Equal(f.Compile, win32All) || f.Defaults != 4 || f.Directive != 4 {
			t.Errorf("flags %q (Defaults %d, Directive %d), want %q", f.Compile, f.Defaults, f.Directive, win32All)
		}
	})
	t.Run("linux", func(t *testing.T) {
		if f := read(t, "linux", map[string]string{"main.gp": winMain}); len(f.Compile) != 0 || f.Defaults != 0 {
			t.Errorf("flags %q on linux, want none", f.Compile)
		}
	})
	t.Run("windows without a header", func(t *testing.T) {
		src := "package main\n\nfunc main() {}\n"
		if f := read(t, "windows", map[string]string{"main.gp": src}); len(f.Compile) != 0 {
			t.Errorf("flags %q for a package with no header and no C file, want none", f.Compile)
		}
	})
	t.Run("windows with a C file", func(t *testing.T) {
		src := "package main\n\nfunc main() {}\n"
		f := read(t, "windows", map[string]string{"main.gp": src, "x.c": "int x;\n"})
		if !slices.Equal(f.Compile, win32All) {
			t.Errorf("flags %q for a package with a C file, want %q", f.Compile, win32All)
		}
	})
	t.Run("the directive", func(t *testing.T) {
		src := "//gocpp:no-win32-defaults\n\n" + winMain
		if f := read(t, "windows", map[string]string{"main.gp": src}); len(f.Compile) != 0 {
			t.Errorf("flags %q with //gocpp:no-win32-defaults, want none", f.Compile)
		}
	})
	t.Run("the user's flags win", func(t *testing.T) {
		src := "//gocpp:cflags -UNOMINMAX -DUNICODE=0 -D _UNICODE -DFAST\n\n" + winMain
		f := read(t, "windows", map[string]string{"main.gp": src})
		want := []string{"-DWIN32_LEAN_AND_MEAN", "-UNOMINMAX", "-DUNICODE=0", "-D", "_UNICODE", "-DFAST"}
		if !slices.Equal(f.Compile, want) || f.Defaults != 1 || f.Directive != len(want) {
			t.Errorf("flags %q (Defaults %d, Directive %d), want %q", f.Compile, f.Defaults, f.Directive, want)
		}
	})
	t.Run("the environment", func(t *testing.T) {
		t.Setenv("GOCPP_CXXFLAGS", "-UWIN32_LEAN_AND_MEAN -O2")
		t.Setenv("GOCPP_CFLAGS", "-DNOMINMAX=1")
		f := read(t, "windows", map[string]string{"main.gp": winMain})
		if want := []string{"-DUNICODE", "-D_UNICODE"}; !slices.Equal(f.Compile, want) {
			t.Errorf("flags %q, want %q", f.Compile, want)
		}
	})
}
