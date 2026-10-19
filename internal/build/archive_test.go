package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests of the archiver search (archive.go) with fake tools in folders of the test: nothing of
// the machine's own tools is used, since PATH is the test's folder.

// toolDir makes a folder with executable files of the given names (each a script that prints its name).
func toolDir(t *testing.T, names ...string) string {
	t.Helper()
	skipWindows(t)
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\necho "+n+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// compilerIn makes a fake compiler in dir that answers -print-file-name=<name> with files[name]
// and -### with the text hash.
func compilerIn(t *testing.T, dir, name string, family Family, files map[string]string, hash string, progs ...map[string]string) CXX {
	t.Helper()
	var progs0 map[string]string
	if len(progs) > 0 {
		progs0 = progs[0]
	}
	var body strings.Builder
	body.WriteString("#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in\n")
	for n, p := range files {
		body.WriteString("-print-file-name=" + n + ") echo '" + p + "'; exit 0;;\n")
	}
	for n, p := range progs0 {
		body.WriteString("-print-prog-name=" + n + ") echo '" + p + "'; exit 0;;\n")
	}
	body.WriteString("-print-file-name=*) echo \"${a#-print-file-name=}\"; exit 0;;\n")
	body.WriteString("-###) echo '" + hash + "' >&2; exit 1;;\n")
	body.WriteString("esac; done\nexit 0\n")
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return CXX{Path: path, Family: family, Version: "15", Major: 15}
}

func isolate(t *testing.T, dirs ...string) {
	t.Helper()
	t.Setenv("GOCPP_AR", "")
	t.Setenv("PATH", strings.Join(dirs, string(os.PathListSeparator)))
}

func TestFindArchiverEnv(t *testing.T) {
	skipWindows(t)
	dir := toolDir(t, "myar")
	t.Setenv("PATH", dir)
	for _, c := range []struct {
		env     string
		path    string
		args    []string
		wantErr string
	}{
		{env: filepath.Join(dir, "myar"), path: filepath.Join(dir, "myar")},
		{env: "myar", path: filepath.Join(dir, "myar")},
		{env: filepath.Join(dir, "myar") + " --plugin /x/p.so", path: filepath.Join(dir, "myar"), args: []string{"--plugin", "/x/p.so"}},
		{env: "'" + filepath.Join(dir, "myar") + "' -v", path: filepath.Join(dir, "myar"), args: []string{"-v"}},
		{env: "nosuchar", wantErr: "nosuchar is not in PATH"},
		{env: "/no/such/dir/ar", wantErr: "is not an executable file"},
		{env: "'unterminated", wantErr: "not a command"},
	} {
		t.Setenv("GOCPP_AR", c.env)
		for _, lto := range []bool{false, true} { // GOCPP_AR replaces the search, with and without LTO
			a, err := FindArchiver(context.Background(), CXX{Path: "/nowhere/g++", Family: GCC}, lto)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("GOCPP_AR=%q: %v, want an error with %q", c.env, err, c.wantErr)
				}
				continue
			}
			if err != nil || a.Path != c.path || strings.Join(a.Args, " ") != strings.Join(c.args, " ") || a.Name != "GOCPP_AR" {
				t.Errorf("GOCPP_AR=%q lto=%v: %+v, %v", c.env, lto, a, err)
			}
		}
	}
}

func TestFindArchiverPlain(t *testing.T) {
	// Next to the compiler first (its own binutils), then PATH; with a target prefix its ar.
	t.Run("next to the compiler", func(t *testing.T) {
		beside := toolDir(t, "ar")
		other := toolDir(t, "ar")
		isolate(t, other)
		cxx := CXX{Path: filepath.Join(beside, "g++"), Family: GCC}
		a, err := FindArchiver(context.Background(), cxx, false)
		if err != nil || a.Path != filepath.Join(beside, "ar") || a.Name != "ar" {
			t.Errorf("%+v, %v; want the ar next to the compiler", a, err)
		}
	})
	t.Run("in PATH", func(t *testing.T) {
		bin := toolDir(t, "ar")
		isolate(t, bin)
		a, err := FindArchiver(context.Background(), CXX{Path: filepath.Join(t.TempDir(), "clang++"), Family: Clang}, false)
		if err != nil || a.Path != filepath.Join(bin, "ar") {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("target prefix", func(t *testing.T) {
		beside := toolDir(t, "x86_64-w64-mingw32-ar", "ar")
		isolate(t)
		cxx := CXX{Path: filepath.Join(beside, "x86_64-w64-mingw32-g++"), Family: GCC}
		a, err := FindArchiver(context.Background(), cxx, false)
		if err != nil || filepath.Base(a.Path) != "x86_64-w64-mingw32-ar" {
			t.Errorf("%+v, %v; want the prefixed ar", a, err)
		}
	})
	t.Run("prefix without prefixed binutils", func(t *testing.T) {
		beside := toolDir(t, "ar")
		isolate(t)
		cxx := CXX{Path: filepath.Join(beside, "aarch64-linux-gnu-g++"), Family: GCC}
		a, err := FindArchiver(context.Background(), cxx, false)
		if err != nil || filepath.Base(a.Path) != "ar" {
			t.Errorf("%+v, %v; want the plain ar", a, err)
		}
	})
	t.Run("none", func(t *testing.T) {
		isolate(t, t.TempDir())
		_, err := FindArchiver(context.Background(), CXX{Path: filepath.Join(t.TempDir(), "g++"), Family: GCC}, false)
		if err == nil || !strings.Contains(err.Error(), "no archiver") {
			t.Errorf("%v", err)
		}
	})
	t.Run("through a symbolic link", func(t *testing.T) {
		// The compiler is a link to a folder with its own tools (a Nix profile, /usr/bin/g++).
		real := toolDir(t, "ar")
		if err := os.WriteFile(filepath.Join(real, "g++"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		links := t.TempDir()
		if err := os.Symlink(filepath.Join(real, "g++"), filepath.Join(links, "g++")); err != nil {
			t.Skip("no symbolic links:", err)
		}
		isolate(t)
		a, err := FindArchiver(context.Background(), CXX{Path: filepath.Join(links, "g++"), Family: GCC}, false)
		if err != nil || a.Path != filepath.Join(real, "ar") {
			t.Errorf("%+v, %v; want the ar of the folder the link leads to", a, err)
		}
	})
}

func TestFindArchiverLTOGCC(t *testing.T) {
	t.Run("gcc-ar", func(t *testing.T) {
		dir := toolDir(t, "gcc-ar", "ar")
		isolate(t)
		cxx := compilerIn(t, dir, "g++", GCC, nil, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "gcc-ar" || filepath.Base(a.Path) != "gcc-ar" || len(a.Args) != 0 {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("versioned gcc-ar", func(t *testing.T) {
		dir := toolDir(t, "gcc-ar-15", "ar")
		isolate(t)
		cxx := compilerIn(t, dir, "g++-15", GCC, nil, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || filepath.Base(a.Path) != "gcc-ar-15" {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("ar with the plugin", func(t *testing.T) {
		dir := toolDir(t, "ar")
		plugin := filepath.Join(t.TempDir(), "liblto_plugin.so")
		os.WriteFile(plugin, []byte("x"), 0o644)
		isolate(t)
		cxx := compilerIn(t, dir, "g++", GCC, map[string]string{"liblto_plugin.so": plugin}, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "ar+plugin" || strings.Join(a.Args, " ") != "--plugin "+plugin {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("plugin found as a program (libexec)", func(t *testing.T) {
		// g++ prints the bare name for -print-file-name=liblto_plugin.so, and the path for
		// -print-prog-name (a Nix wrapper has no gcc-ar next to it, either).
		dir := toolDir(t, "ar")
		plugin := filepath.Join(t.TempDir(), "liblto_plugin.so")
		os.WriteFile(plugin, []byte("x"), 0o644)
		isolate(t)
		cxx := compilerIn(t, dir, "g++", GCC, nil, "", map[string]string{"liblto_plugin.so": plugin})
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "ar+plugin" || strings.Join(a.Args, " ") != "--plugin "+plugin {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("a name that only looks like a target prefix", func(t *testing.T) {
		// my-g++ (a symbolic link to g++) has no my-gcc-ar: the unprefixed gcc-ar is the one.
		dir := toolDir(t, "gcc-ar")
		isolate(t)
		cxx := compilerIn(t, dir, "my-g++", GCC, nil, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "gcc-ar" || filepath.Base(a.Path) != "gcc-ar" {
			t.Errorf("%+v, %v", a, err)
		}
		// Only ar and the plugin: ar with --plugin.
		dir = toolDir(t, "ar")
		plugin := filepath.Join(t.TempDir(), "liblto_plugin.so")
		os.WriteFile(plugin, []byte("x"), 0o644)
		cxx = compilerIn(t, dir, "my-g++", GCC, map[string]string{"liblto_plugin.so": plugin}, "")
		a, err = FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "ar+plugin" || filepath.Base(a.Path) != "ar" {
			t.Errorf("%+v, %v", a, err)
		}
		// clang: the plugin with the unprefixed ar.
		cl := filepath.Join(t.TempDir(), "LLVMgold.so")
		os.WriteFile(cl, []byte("x"), 0o644)
		cxx = compilerIn(t, dir, "my-clang++", Clang, map[string]string{"LLVMgold.so": cl}, "")
		a, err = FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "ar+plugin" || filepath.Base(a.Path) != "ar" {
			t.Errorf("clang: %+v, %v", a, err)
		}
	})
	t.Run("plain ar is refused", func(t *testing.T) {
		// A compiler that has no plugin file, and only a plain ar: the error says what is missing.
		dir := toolDir(t, "ar")
		isolate(t)
		cxx := compilerIn(t, dir, "g++", GCC, nil, "")
		_, err := FindArchiver(context.Background(), cxx, true)
		if err == nil {
			t.Fatal("a plain ar was taken for LTO")
		}
		for _, want := range []string{"-lto needs an archiver", "gcc-ar", "plugin needed to handle lto object", "GOCPP_AR", "g++"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the error does not say %q: %v", want, err)
			}
		}
	})
	t.Run("a plugin that does not exist", func(t *testing.T) {
		dir := toolDir(t, "ar")
		isolate(t)
		cxx := compilerIn(t, dir, "g++", GCC, map[string]string{"liblto_plugin.so": "/no/such/plugin.so"}, "")
		if _, err := FindArchiver(context.Background(), cxx, true); err == nil {
			t.Error("a missing plugin file was accepted")
		}
	})
}

func TestFindArchiverLTOClang(t *testing.T) {
	t.Run("llvm-ar", func(t *testing.T) {
		dir := toolDir(t, "llvm-ar", "ar")
		isolate(t)
		cxx := compilerIn(t, dir, "clang++", Clang, nil, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "llvm-ar" {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("llvm-ar in PATH", func(t *testing.T) {
		bin := toolDir(t, "llvm-ar")
		isolate(t, bin)
		cxx := compilerIn(t, t.TempDir(), "clang++", Clang, nil, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Path != filepath.Join(bin, "llvm-ar") {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("versioned llvm-ar", func(t *testing.T) {
		bin := toolDir(t, "llvm-ar-15")
		isolate(t, bin)
		cxx := compilerIn(t, t.TempDir(), "clang++", Clang, nil, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || filepath.Base(a.Path) != "llvm-ar-15" {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("gold plugin from -print-file-name", func(t *testing.T) {
		dir := toolDir(t, "ar")
		plugin := filepath.Join(t.TempDir(), "LLVMgold.so")
		os.WriteFile(plugin, []byte("x"), 0o644)
		isolate(t)
		cxx := compilerIn(t, dir, "clang++", Clang, map[string]string{"LLVMgold.so": plugin}, "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "ar+plugin" || strings.Join(a.Args, " ") != "--plugin "+plugin {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("plugin from the link command that -### prints", func(t *testing.T) {
		// A Nix wrapper puts the plugin only in the link command.
		dir := toolDir(t, "ar")
		plugin := filepath.Join(t.TempDir(), "LLVMgold.so")
		os.WriteFile(plugin, []byte("x"), 0o644)
		isolate(t)
		cxx := compilerIn(t, dir, "clang++", Clang, nil, `"/usr/bin/ld" "-plugin" "`+plugin+`" "-o" "a.out"`)
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil || a.Name != "ar+plugin" || strings.Join(a.Args, " ") != "--plugin "+plugin {
			t.Errorf("%+v, %v", a, err)
		}
	})
	t.Run("plain ar is refused", func(t *testing.T) {
		dir := toolDir(t, "ar")
		isolate(t)
		cxx := compilerIn(t, dir, "clang++", Clang, nil, "")
		_, err := FindArchiver(context.Background(), cxx, true)
		if err == nil || !strings.Contains(err.Error(), "llvm-ar") || !strings.Contains(err.Error(), "GOCPP_AR") {
			t.Errorf("%v; want an error that names llvm-ar and GOCPP_AR", err)
		}
	})
}

func TestFindArchiverLTOUnknownFamily(t *testing.T) {
	isolate(t, toolDir(t, "ar", "gcc-ar", "llvm-ar"))
	_, err := FindArchiver(context.Background(), CXX{Path: "/x/mycc"}, true)
	if err == nil || !strings.Contains(err.Error(), "no LTO archiver is known") {
		t.Errorf("%v", err)
	}
}

func TestArchiverIdentity(t *testing.T) {
	skipWindows(t)
	dir := toolDir(t, "ar")
	p := filepath.Join(dir, "ar")
	a := Archiver{Path: p, Name: "ar"}
	id1 := strings.Join(a.identity(), "|")
	if again := strings.Join(a.identity(), "|"); again != id1 {
		t.Error("the identity changes without a change")
	}
	for name, other := range map[string]Archiver{
		"name": {Path: p, Name: "gcc-ar"},
		"args": {Path: p, Name: "ar", Args: []string{"--plugin", "/x"}},
		"path": {Path: p + "2", Name: "ar"},
	} {
		if strings.Join(other.identity(), "|") == id1 {
			t.Errorf("another %s, the same identity", name)
		}
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho a different archiver\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if strings.Join(a.identity(), "|") == id1 {
		t.Error("the archiver's file changed, the identity did not")
	}
}
