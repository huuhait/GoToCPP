package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
)

// TestApostropheInFileName builds a program with a file name that has an apostrophe. The same
// name was a fixture in tests/pkgs/odd_file_names, but a file name with an apostrophe is not allowed
// in a Go module zip, so a repository that has one can't be installed with go install (step 15.13,
// TestGoInstallOfATaggedVersion in internal/packaging): the file is made here, in a temporary folder.
func TestApostropheInFileName(t *testing.T) {
	anyCXX(t)
	t.Setenv("GOCPP_TEST_PHASE", "06")
	inDir(t, map[string]string{
		"gocpp.mod":     "module example.com/quote\n",
		"main.gp":       "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/quote/lib\"\n)\n\nfunc main() { fmt.Println(lib.Q()) }\n",
		"lib/q'uote.gp": "package lib\n\nfunc Q() int { return 7 }\n",
	})
	if r := gocppRun("", false, "run", "."); r != (result{0, "7\n", ""}) {
		t.Errorf("gocpp run . = %+v, want exit 0 and output 7", r)
	}
}

// TestOddFolders builds and runs a program with the project, the cache and the temporary folder
// in folders whose names a user can have: letters outside ASCII and spaces (a project in
// ~/проект, C:\Users\Zoë), and a double quote. Before, Nix's clang wrapper failed on every
// non-ASCII path, and g++'s assembler failed on a quote in the path of a file with inline asm
// (the runtime has some; the quote case builds it with the sanitizers, whose build of the
// runtime has it). The runtime is built into the folder of the cache, so it is built again.
func TestOddFolders(t *testing.T) {
	cases := []struct {
		name                string
		project, cache, tmp string
		flags               []string
		skipOnWindows       bool
	}{
		{"non-ASCII", "проект ёж", "кэш Zoë", "tmp é", nil, false},
		{"quote", `q"uote`, `ca"ch`, `tm"p`, []string{"-asan"}, true},
		// -release is -O2 -flto: the units with a quote in their path are compiled without LTO.
		{"quote release", `q"uote`, `ca"ch`, `tm"p`, []string{"-release"}, true},
	}
	forEachCXX(t, func(t *testing.T, c testCXX) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.skipOnWindows && runtime.GOOS == "windows" {
					t.Skip("a double quote is not allowed in a Windows file name")
				}
				t.Setenv("GOCPP_TEST_PHASE", "06")
				root := t.TempDir()
				project := filepath.Join(root, tc.project)
				for _, dir := range []string{project, filepath.Join(root, tc.cache), filepath.Join(root, tc.tmp)} {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				// A C++ file of the project with inline asm (g++ writes a line marker with the
				// file's name for it), and a C file: the compiles of the project's own sources.
				writeFiles(t, project, map[string]string{
					"main.gp":  oddFoldersMain,
					"util.hpp": "int twice(int x);\n",
					"util.cpp": "#include \"util.hpp\"\nint twice(int x) {\n\tasm volatile(\"nop\");\n\treturn 2 * x;\n}\n",
					"cutil.h":  "int thrice(int x);\n",
					"cutil.c":  "#include \"cutil.h\"\nint thrice(int x) { return 3 * x; }\n",
				})
				t.Chdir(project)
				t.Setenv("XDG_CACHE_HOME", filepath.Join(root, tc.cache))
				t.Setenv("TMPDIR", filepath.Join(root, tc.tmp))
				t.Setenv("GOCPP_LSP_CACHE", filepath.Join(root, tc.cache, "gocpp-lsp"))
				args := append(append([]string{"run"}, tc.flags...), ".")
				want := helloOut + "42 63\n"
				if r := gocppRun("", true, args...); r != (result{0, want, ""}) {
					t.Errorf("gocpp %s = %+v, want exit 0 and %q", strings.Join(args, " "), r, want)
				}
			})
		}
	})
}

// oddFoldersMain is helloSrc that also calls a C++ function and a C function of the project.
const oddFoldersMain = `package main

import (
	"fmt"

	"./cutil.h"
	"./util.hpp"
)

func add(a, b int) (int, bool) {
	return a + b, a+b > 10
}

func main() {
	s, big := add(7, 5)
	fmt.Println("hello, gocpp", s, big)
	println("to stderr", s)
	fmt.Println("after")
	fmt.Println(int(util.twice(21)), int(cutil.thrice(21)))
}
`

// TestOddFoldersWindows cross-builds from a project, cache and temporary folder with non-ASCII
// letters and spaces for windows/amd64 (MinGW's windres and ld got the paths), and runs a program
// with a non-ASCII name on the real Windows PC of GOCPP_WINDOWS_SSH=user@host (never Wine).
// Skipped without the variable; with it, an unreachable PC is a failure.
func TestOddFoldersWindows(t *testing.T) {
	host := os.Getenv("GOCPP_WINDOWS_SSH")
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host runs the programs on the real Windows PC")
	}
	remote, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is unreachable: %v", err)
	}
	defer remote.Close()
	root := t.TempDir()
	project := filepath.Join(root, "проект Zoë")
	for _, dir := range []string{project, filepath.Join(root, "кэш"), filepath.Join(root, "tmp é")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFiles(t, project, map[string]string{"main.gp": helloSrc})
	t.Chdir(project)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "кэш"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp é"))
	t.Setenv("GOCPP_LSP_CACHE", filepath.Join(root, "кэш", "gocpp-lsp"))
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	exe := filepath.Join(root, "программа Zoë.exe")
	if r := gocppRun("", false, "build", "-os", "windows", "-arch", "amd64", "-o", exe, "."); r.code != 0 {
		t.Fatalf("build for windows: exit %d\n%s", r.code, r.stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	res, err := remote.Run(ctx, 2*time.Minute, exe, nil, nil, false)
	if err != nil {
		t.Fatalf("running on the PC: %v", err)
	}
	got := strings.ReplaceAll(string(res.Stdout)+string(res.Stderr), "\r\n", "\n")
	if res.Code != 0 || !strings.Contains(got, "hello, gocpp 12 true\n") {
		t.Fatalf("on the PC: exit %d\nstderr:\n%s\nstdout:\n%s", res.Code, res.Stderr, res.Stdout)
	}
}
