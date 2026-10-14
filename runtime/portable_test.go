package rtfiles_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// mingwCXX is the MinGW-w64 cross compiler of the dev shell (flake.nix).
const mingwCXX = "x86_64-w64-mingw32-g++"

// TestWindowsSyntax compiles the portable C++ of the runtime (everything outside src/os/)
// for Windows with MinGW-w64, syntax only. It backs up TestNoOSHeadersOutsideOS, which sees
// only #include lines: a POSIX function that glibc's standard headers declare anyway (g++
// defines _GNU_SOURCE, so <cstdio> declares fileno) or a hand-written OS declaration
// doesn't compile for Windows. It is skipped when the cross compiler isn't installed.
func TestWindowsSyntax(t *testing.T) {
	cc, err := build.LookCXX(t.Context(), mingwCXX)
	if err != nil {
		t.Skipf("no MinGW-w64 cross compiler: %v", err)
	}
	var files []string
	disk := os.DirFS(".")
	err = fs.WalkDir(disk, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipEntry(disk, p, d) {
			return skipped(d)
		}
		if d.IsDir() && p == "src/os" {
			return fs.SkipDir
		}
		if !d.IsDir() && path.Ext(p) == ".cpp" {
			files = append(files, sourcePath(t, p))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		args := append(slices.Clone(baseFlags), "-O0", "-fsyntax-only", "-I", "include", f)
		out, _, code, err := command(".", build.CompilerEnv(), true, cc.Path, args...)
		if err != nil || code != 0 || len(out) != 0 {
			t.Errorf("%s %s: exit %d %v\n%s", cc.Path, strings.Join(args, " "), code, err, out)
		}
	}
	if len(files) == 0 {
		t.Fatal("found no .cpp files outside src/os/")
	}
}

// TestClangFormat checks that the runtime's C++ follows the repo's .clang-format, with the
// dev shell's clang-format (version 21; other versions can format some lines differently, so
// they skip, as does a missing clang-format).
func TestClangFormat(t *testing.T) {
	cf, err := exec.LookPath("clang-format")
	if err != nil {
		t.Skipf("no clang-format: %v", err)
	}
	v, err := exec.Command(cf, "--version").Output()
	if err != nil {
		t.Fatalf("clang-format --version: %v", err)
	}
	if !strings.Contains(string(v), "version 21.") {
		t.Skipf("clang-format is %s; the style is checked with version 21", strings.TrimSpace(string(v)))
	}
	if _, err := os.ReadFile("../.clang-format"); err != nil {
		t.Fatal(err)
	}
	var files []string
	disk := os.DirFS(".")
	err = fs.WalkDir(disk, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skipEntry(disk, p, d) {
			return skipped(d)
		}
		if d.IsDir() {
			return nil
		}
		switch path.Ext(p) {
		case ".h", ".hpp", ".cpp":
			files = append(files, filepath.FromSlash(p))
			sourcePath(t, p) // for go test's cache
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out, _, code, err := command(".", nil, true, cf, append([]string{"--dry-run", "--Werror"}, files...)...)
	if err != nil || code != 0 {
		t.Errorf("clang-format --dry-run --Werror: exit %d %v; run clang-format -i on these files:\n%s", code, err, out)
	}
}
