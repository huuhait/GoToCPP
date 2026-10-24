package main

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// TestIncludedHeaderNamedPipe: a named pipe that the C/C++ compiler reaches through an #include
// (of a header that a .gp file imports, of a package's .c file, or found through a package's -I
// folder) is an error that names it, not a compiler that waits for a writer for ever: the probes
// of gocpp check and the compiles of gocpp build are stopped (build.WatchPipes). The import has
// one error.
func TestIncludedHeaderNamedPipe(t *testing.T) {
	const twice = "static inline int twice(int x) { return 2 * x; }\n"
	for _, c := range []struct {
		name     string
		files    map[string]string
		pipe     string
		commands []string
	}{
		{"imported-header", map[string]string{
			"main.gp": "package main\n\nimport \"./a.h\"\n\nfunc main() {\n\tprintln(a.twice(2))\n}\n",
			"a.h":     "#include \"pipe.h\"\n" + twice,
		}, "pipe.h", []string{"check", "build"}},
		{"c-file", map[string]string{
			"main.gp": "package main\n\nimport \"./a.h\"\n\nfunc main() {\n\tprintln(a.twice(2))\n}\n",
			"a.h":     "int twice(int x);\n",
			"a.c":     "#include \"a.h\"\n#include \"pipe.h\"\nint twice(int x) { return 2 * x; }\n",
		}, "pipe.h", []string{"build"}},
		// The compiler prints a warning before it opens the pipe: the pipe's line still comes,
		// after the warning (ccache would hold the warning back: CCACHE_DISABLE).
		{"c-file-warning", map[string]string{
			"main.gp": "package main\n\nimport \"./a.h\"\n\nfunc main() {\n\tprintln(a.twice(2))\n}\n",
			"a.h":     "int twice(int x);\n",
			"a.c":     "#include \"a.h\"\n#warning \"before the pipe\"\n#include \"pipe.h\"\nint twice(int x) { return 2 * x; }\n",
		}, "pipe.h", []string{"build"}},
		{"through-I", map[string]string{
			"main.gp": "//gocpp:cflags -Iinc\npackage main\n\nimport \"./a.h\"\n\nfunc main() {\n\tprintln(a.twice(2))\n}\n",
			"a.h":     "#include <deep.h>\n" + twice,
			"inc/ok":  "",
		}, "inc/deep.h", []string{"check", "build"}},
	} {
		for _, command := range c.commands {
			t.Run(c.name+"/"+command, func(t *testing.T) {
				t.Setenv("CCACHE_DISABLE", "1")
				files := map[string]string{"gocpp.mod": "module example.com/m\n"}
				for name, text := range c.files {
					files[name] = text
				}
				dir := inDir(t, files)
				pipe := filepath.Join(dir, filepath.FromSlash(c.pipe))
				if err := syscall.Mkfifo(pipe, 0o644); err != nil {
					t.Skip(err)
				}
				got := runGuarded(t, []string{pipe}, command, ".")
				if got.code == 0 || !strings.Contains(got.stderr, filepath.Base(pipe)+": not a regular file: it is a named pipe") {
					t.Fatalf("gocpp %s .: exit %d, stderr %q; want an error about the named pipe %s", command, got.code, got.stderr, c.pipe)
				}
				switch {
				case strings.HasPrefix(c.name, "c-file"):
					// The file that was compiled, then the pipe: no command, no "printed nothing".
					if !strings.Contains(got.stderr, "\n./a.c: open ./pipe.h: not a regular file: it is a named pipe (the compiler waited") ||
						strings.Contains(got.stderr, "printed nothing") || strings.Contains(got.stderr, " -c ") {
						t.Errorf("gocpp %s .: stderr %q; want the line ./a.c: open ./pipe.h: ... alone", command, got.stderr)
					}
					if c.name == "c-file-warning" && !strings.Contains(got.stderr, "before the pipe") {
						t.Errorf("gocpp %s .: stderr %q; want the warning too", command, got.stderr)
					}
				case strings.Count(got.stderr, "./main.gp:") != 1:
					t.Errorf("gocpp %s .: stderr %q; want one error at the import", command, got.stderr)
				}
			})
		}
	}
}

// TestNamedPipeInBuildFiles: the other compiles that reach a named pipe say so too: a source of a
// module's C library (the recipe's error names the source, then the pipe), and the generated C++
// with an -include flag of the package (once, not once per unit, and not as a bug of gocpp).
func TestNamedPipeInBuildFiles(t *testing.T) {
	t.Setenv("CCACHE_DISABLE", "1")
	t.Run("library", func(t *testing.T) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		dir := inDir(t, libProject("\tinclude include\n\tsrc src/ok.c\n\tsrc src/bad.c\n", "\tfmt.Println(clib.clib_square(2))",
			map[string]string{"include/clib.h": clibH, "src/ok.c": "int clib_square(int x) { return x * x; }\n",
				"src/bad.c": "#include \"pipe.h\"\nint clib_scaled(int x) { return x; }\n"}))
		pipe := filepath.Join(dir, "clib", "src", "pipe.h")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		got := runGuarded(t, []string{pipe}, "build", "-o", filepath.Join(dir, "prog"), ".")
		for _, want := range []string{
			"src/bad.c did not compile (recipe ./gocpp.mod:9: src src/bad.c)",
			"\nsrc/bad.c: open " + pipe + ": not a regular file: it is a named pipe (the compiler waited",
		} {
			if got.code != 1 || !strings.Contains(got.stderr, want) {
				t.Errorf("gocpp build: exit %d, stderr %q; want exit 1 and %q", got.code, got.stderr, want)
			}
		}
	})
	t.Run("include-flag", func(t *testing.T) {
		dir := inDir(t, map[string]string{
			"gocpp.mod": "module example.com/m\n",
			"main.gp":   "//gocpp:cflags -include pipe.h\npackage main\n\nfunc main() {\n\tprintln(1)\n}\n",
		})
		pipe := filepath.Join(dir, "pipe.h")
		if err := syscall.Mkfifo(pipe, 0o644); err != nil {
			t.Skip(err)
		}
		got := runGuarded(t, []string{pipe}, "build", "-o", filepath.Join(dir, "prog"), ".")
		const want = "the generated C++: open ./pipe.h: not a regular file: it is a named pipe (the compiler waited"
		if got.code != 1 || strings.Count(got.stderr, want) != 1 || strings.Contains(got.stderr, "bug in gocpp") {
			t.Errorf("gocpp build: exit %d, stderr %q; want exit 1 and %q once", got.code, got.stderr, want)
		}
	})
}

// TestPipeErrorIsNotEndedFromOutside: a compile that build.WatchPipes stopped is reported as the
// named pipe, also when the compiler's driver printed that its program was ended by a signal
// (gocpp's own SIGTERM): the cause is known, so it is not "ended by the signal".
func TestPipeErrorIsNotEndedFromOutside(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	d := &driver{name: "build", stderr: &out, check: &checkCommand{dir: dir}}
	pkg := &check.Package{Load: &load.Package{ImportPath: "example.com/m", Dir: dir}}
	ce := &build.CompileError{What: "the C file a.c", Source: filepath.Join(dir, "a.c"),
		Output: []byte("gcc: fatal error: Terminated signal terminated program cc1\n"),
		Err:    &build.PipeError{Path: filepath.Join(dir, "pipe.h")}}
	if _, ok := ce.EndedFromOutside(); !ok {
		t.Fatal("the output does not look ended from outside: the test checks nothing")
	}
	code, _ := d.reportFailure(&buildSet{main: pkg}, ce, build.EnvFlags{}, t.TempDir())
	if code != exitErrors || !strings.Contains(out.String(), "./a.c: open ./pipe.h: not a regular file: it is a named pipe") ||
		strings.Contains(out.String(), "was ended by the signal") {
		t.Errorf("reportFailure = %d, printed %q; want exit %d and the named pipe", code, out.String(), exitErrors)
	}
}
