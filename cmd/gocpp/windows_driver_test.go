package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// The naming rules of a Windows target (Phase 14 step 14.2), checked on every system.

func TestWithExeSuffix(t *testing.T) {
	for _, c := range []struct{ out, goos, want string }{
		{"prog", "windows", "prog.exe"},
		{"prog.exe", "windows", "prog.exe"},
		{"PROG.EXE", "windows", "PROG.EXE"},
		{"prog.bin", "windows", "prog.bin"}, // a name with an extension keeps it
		{filepath.Join("out", "prog"), "windows", filepath.Join("out", "prog.exe")},
		{"prog", "linux", "prog"},
		{"prog", "darwin", "prog"},
	} {
		if got := withExeSuffix(c.out, c.goos); got != c.want {
			t.Errorf("withExeSuffix(%q, %s) = %q, want %q", c.out, c.goos, got, c.want)
		}
	}
}

func TestOutputPathForWindows(t *testing.T) {
	dir := t.TempDir()
	d := &driver{flags: &buildFlags{output: "prog"}, check: &checkCommand{dir: dir, goos: "windows"}}
	if got, want := d.outputPath("hello.exe"), filepath.Join(dir, "prog.exe"); got != want {
		t.Errorf("-o prog for windows = %q, want %q", got, want)
	}
	d.flags.output = ""
	if got, want := d.outputPath("hello.exe"), filepath.Join(dir, "hello.exe"); got != want {
		t.Errorf("no -o = %q, want %q", got, want)
	}
	d.check.goos = "linux"
	d.flags.output = "prog"
	if got, want := d.outputPath("hello"), filepath.Join(dir, "prog"); got != want {
		t.Errorf("-o prog for linux = %q, want %q", got, want)
	}
}

func TestLibraryNamesForWindows(t *testing.T) {
	dir := t.TempDir()
	pkg := &check.Package{Load: &load.Package{Files: []*load.File{{Path: filepath.Join(dir, "hello.gp")}}}}
	for _, c := range []struct {
		goos, mode, out, name, file string
	}{
		{"windows", "c-shared", "", "hello", "hello.dll"},
		{"windows", "c-shared", "mylib.dll", "mylib", "mylib.dll"},
		{"windows", "c-shared", "mylib", "mylib", "mylib.dll"}, // no extension: .dll, as -o prog gives prog.exe
		{"windows", "c-shared", "MyLib.DLL", "MyLib", "MyLib.DLL"},
		{"windows", "c-archive", "mylib", "mylib", "mylib.a"},
		{"windows", "c-archive", "libmy.a", "libmy", "libmy.a"},
		{"linux", "c-shared", "mylib", "mylib", "mylib"}, // Linux names the file as asked
		{"windows", "c-archive", "", "libhello", "libhello.a"},
		{"linux", "c-shared", "", "libhello", "libhello.so"},
	} {
		d := &driver{files: true, flags: &buildFlags{output: c.out, mode: build.Mode(c.mode)}, check: &checkCommand{dir: dir, goos: c.goos}}
		name, file, err := d.libraryNames(pkg)
		if err != nil {
			t.Errorf("%s %s -o %q: %v", c.goos, c.mode, c.out, err)
			continue
		}
		if name != c.name || file != c.file {
			t.Errorf("%s %s -o %q: name %q file %q, want %q %q", c.goos, c.mode, c.out, name, file, c.name, c.file)
		}
	}
}

func TestLibraryNamesForWindowsRefuseSo(t *testing.T) {
	dir := t.TempDir()
	pkg := &check.Package{Load: &load.Package{Files: []*load.File{{Path: filepath.Join(dir, "hello.gp")}}}}
	for _, c := range []struct{ out, suggest string }{{"mylib.so", "mylib.dll"}, {"libmy.dylib", "libmy.dll"}, {"z.dll.a", "z.dll"}, {"z.lib", "z.dll"}} {
		out := c.out
		d := &driver{files: true, flags: &buildFlags{output: out, mode: build.ModeCShared}, check: &checkCommand{dir: dir, goos: "windows"}}
		_, _, err := d.libraryNames(pkg)
		if err == nil || !strings.Contains(err.Error(), "a shared library for windows is a .dll") || !strings.Contains(err.Error(), "(name it "+c.suggest+",") {
			t.Errorf("-o %s: error = %v, want a suggestion of %s", out, err, c.suggest)
		}
	}
}
