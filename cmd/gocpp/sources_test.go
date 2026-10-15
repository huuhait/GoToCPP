package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests of a package's own C and C++ files in gocpp build and run
// (Phase 04, step 4.13). The generated C++ calls them through testHookCPP
// (callSources); externc_test.go calls C functions from .gp code.

// callSources sets testHookCPP for the rest of the test so that the
// program declares the functions decls (C++ declarations) and prints the
// result of the expression call (an int) before main runs. All of it comes
// after the generated C++, which includes gocpp/nofortify.h first.
func callSources(t *testing.T, decls, call string) {
	t.Helper()
	testHookCPP = func(cpp []byte) []byte {
		return []byte(string(cpp) + "\n#include <cstdio>\n" + decls +
			"\nstatic int gp_test_sources = (std::printf(\"sources %d\\n\", " + call + "), std::fflush(stdout), 0);\n")
	}
	t.Cleanup(func() { testHookCPP = nil })
}

// mainSrc is a program that prints "main".
const mainSrc = "package main\n\nfunc main() {\n\tprintln(\"main\")\n}\n"

// The C file of TestBuildSources is GNU C17 and no C++: new is a name,
// malloc's void* converts implicitly, and _Generic is C only. glibc declares
// strdup, fileno and M_PI (it doesn't with the strict -std=c17). It has the
// same base name as the C++ file. It also checks the flags it gets: those
// of the package and GOCPP_CFLAGS, but not GOCPP_CXXFLAGS.
const sourcesC = `#include <math.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "local.h"

#if __STDC_VERSION__ != 201710L
#error not C17 (gnu17)
#endif
#ifdef __STRICT_ANSI__
#error not GNU C
#endif
#if FROM_PKG != 7 || FROM_ENV != 1
#error no package or environment flags
#endif
#ifdef FROM_CXXENV
#error GOCPP_CXXFLAGS in a C compile
#endif

static int new = LOCAL_ANSWER;

int c_answer(void) {
	int *p = malloc(sizeof *p);
	*p = _Generic(new, int: new, default: 0);
	int v = *p;
	free(p);
	char *s = strdup("x");
	v += (int)strlen(s) - 1 + (fileno(stdout) >= 0 ? 0 : 1000) + (int)(M_PI - 3.0);
	free(s);
	return v;
}
`

// The C++ file is C++20, and gets GOCPP_CXXFLAGS too.
const sourcesCXX = `#include <concepts>
#include <span>
#include "local.h"

#if __cplusplus != 202002L
#error not C++20
#endif
#if FROM_PKG != 7 || FROM_ENV != 1 || FROM_CXXENV != 2
#error no package or environment flags
#endif

template <std::integral T>
T twice(T x) { return 2 * x; }

int cxx_answer() {
	int xs[] = {LOCAL_ANSWER, 1};
	std::span<int> s(xs);
	return twice(s[0]) + static_cast<int>(s.size());
}
`

func TestBuildSources(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04") // directives
		t.Setenv("GOCPP_CFLAGS", "-DFROM_ENV=1")
		t.Setenv("GOCPP_CXXFLAGS", "-DFROM_CXXENV=2")
		dir := inDir(t, map[string]string{
			"main.gp": "//gocpp:cflags -DFROM_PKG=7\n//gocpp:link m\n" + mainSrc,
			"a.c":     sourcesC,
			"a.cpp":   sourcesCXX,
			"local.h": "#define LOCAL_ANSWER 20\n",
			// Not for this system, or not a source: never compiled.
			"x_windows.c": "#error windows only\n",
			"ignored.cc":  "//go:build ignore\n\n#error ignored\n",
			"other.c++":   "#error not a source\n",
			"local.hpp":   "#error a header\n",
		})
		callSources(t, `extern "C" int c_answer(); int cxx_answer();`, "c_answer() * 100 + cxx_answer()")
		if r := gocppRun("", true, "run", "."); r != (result{0, "sources 2042\nmain\n", ""}) {
			t.Fatalf("gocpp run . = %+v, want exit 0 and the answers of the C and C++ files", r)
		}

		r := gocppRun("", false, "build", "-x", "-o", filepath.Join(dir, "prog"), ".")
		if r.code != 0 {
			t.Fatalf("gocpp build -x = %+v", r)
		}
		cc, err := build.FindCC(context.Background(), c.cxx)
		if err != nil {
			t.Fatal(err)
		}
		// The C file with the C compiler, the C++ file with the C++
		// compiler, each with its standard, the package folder for "" and
		// gocpp/nofortify.h, then the flags of the user.
		for _, w := range []struct {
			c   build.CXX
			std string
			env string
			src string
		}{
			{cc, "-std=gnu17", "-DFROM_ENV=1", "-x c " + filepath.Join(dir, "a.c") + " -o $WORK/cobj/a.c.o"},
			{c.cxx, "-std=c++20", "-DFROM_ENV=1 -DFROM_CXXENV=2", "-x c++ " + filepath.Join(dir, "a.cpp") + " -o $WORK/cobj/a.cpp.o"},
		} {
			want := regexp.QuoteMeta(w.c.Path+" "+strings.Join(build.SourceFlags(w.c, false), " ")+" "+w.std+" -iquote "+dir+" -include ") +
				`\S+/include/gocpp/nofortify\.h ` + regexp.QuoteMeta(w.env+" -DFROM_PKG=7 -c "+w.src)
			if !regexp.MustCompile(`(?m)^` + want + `$`).MatchString(r.stderr) {
				t.Errorf("-x has no line\n\t%s\nin\n%s", want, r.stderr)
			}
		}
		// The objects go to the link after the generated one and before the
		// runtime's and the libraries.
		if !regexp.MustCompile(`(?m) \$WORK/[^ ]+\.o \$WORK/cobj/a\.c\.o \$WORK/cobj/a\.cpp\.o \S+/rt/.* -lm -o \$WORK/`).MatchString(r.stderr) {
			t.Errorf("the link lacks the objects of the C and C++ files:\n%s", r.stderr)
		}
		// The flags of generated C++ are for generated C++ only.
		for _, line := range strings.Split(r.stderr, "\n") {
			if strings.Contains(line, " -o $WORK/cobj/") &&
				(strings.Contains(line, " -fwrapv") || strings.Contains(line, " -Wno-error") || strings.Contains(line, " -Werror=") || strings.Contains(line, "-Wno-tautological-compare")) {
				t.Errorf("a compile of the user's code has gocpp's flags of generated C++:\n%s", line)
			}
		}

		// gocpp run file.gp builds only that file, like go run x.go: the C
		// and C++ functions are missing then.
		r = gocppRun("", false, "run", "main.gp")
		if m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr); m != nil {
			os.RemoveAll(m[1])
		}
		if r.code == 0 || !strings.Contains(r.stderr, "c_answer") {
			t.Errorf("gocpp run main.gp = %+v, want a link error (the C files are not in a package of named files)", r)
		}
	})
}

func TestBuildSourceErrors(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		// An error in the user's C file: its compiler's output with
		// ./bad.c, exit 1, and no work folder left.
		dir := inDir(t, map[string]string{"main.gp": mainSrc, "ok.cpp": "int ok() { return 1; }\n", "bad.c": "int f(void) {\n\treturn missing;\n}\n"})
		r := gocppRun("", false, "build", ".")
		if r.code != 1 || r.stdout != "" || !strings.HasPrefix(r.stderr, "# command-line-arguments\n") ||
			!regexp.MustCompile(`(?m)^\./bad\.c:2:\d+: error: .*missing`).MatchString(r.stderr) ||
			strings.Contains(r.stderr, dir) || strings.Contains(r.stderr, "internal error") || strings.Contains(r.stderr, "work folder") {
			t.Errorf("gocpp build with a broken C file = %+v, want exit 1 and the error at ./bad.c:2", r)
		}
		os.Remove("bad.c")

		// The same for a C++ file.
		writeFiles(t, dir, map[string]string{"bad.cpp": "#include <vector>\nint g() {\n\tstd::vector<int> v;\n\treturn v.size_bad();\n}\n"})
		r = gocppRun("", false, "build", ".")
		if r.code != 1 || !strings.HasPrefix(r.stderr, "# command-line-arguments\n") ||
			!regexp.MustCompile(`(?m)^\./bad\.cpp:4:\d+: error: .*size_bad`).MatchString(r.stderr) ||
			strings.Contains(r.stderr, dir) || strings.Contains(r.stderr, "internal error") {
			t.Errorf("gocpp build with a broken C++ file = %+v, want exit 1 and the error at ./bad.cpp:4", r)
		}
		os.Remove("bad.cpp")

		// Warnings of files that compile: under the package's line, once,
		// with the short paths, and the build goes on.
		writeFiles(t, dir, map[string]string{"w1.c": "#warning first warning\nint w1;\n", "w2.cpp": "#warning second warning\nint w2;\n"})
		r = gocppRun("", false, "build", "-o", filepath.Join(dir, "prog"), ".")
		if r.code != 0 || !strings.HasPrefix(r.stderr, "# command-line-arguments\n") || strings.Count(r.stderr, "# command-line-arguments") != 1 ||
			!regexp.MustCompile(`(?m)^\./w1\.c:1:\d+: warning: .*first warning`).MatchString(r.stderr) ||
			!regexp.MustCompile(`(?m)^\./w2\.cpp:1:\d+: warning: .*second warning`).MatchString(r.stderr) ||
			strings.Contains(r.stderr, dir) {
			t.Errorf("gocpp build with warnings = %+v, want exit 0 and the warnings under one package line", r)
		}
		// A warning in one file and an error in another: one package line.
		writeFiles(t, dir, map[string]string{"bad.c": "int f(void) {\n\treturn missing;\n}\n"})
		r = gocppRun("", false, "build", ".")
		if r.code != 1 || !strings.HasPrefix(r.stderr, "# command-line-arguments\n") || strings.Count(r.stderr, "# command-line-arguments") != 1 ||
			!strings.Contains(r.stderr, "first warning") || !strings.Contains(r.stderr, "missing") {
			t.Errorf("gocpp build with a warning and an error = %+v, want exit 1 and one package line", r)
		}
		os.Remove("bad.c")
		os.Remove("w1.c")
		os.Remove("w2.cpp")

		// A function that the user's code lacks: a link error of the user.
		callSources(t, "int ok(); int gp_test_missing();", "ok() + gp_test_missing()")
		r = gocppRun("", false, "build", ".")
		if r.code != 1 || !strings.HasPrefix(r.stderr, "# command-line-arguments\nthe program did not link: ") ||
			!strings.Contains(r.stderr, "gp_test_missing") ||
			!strings.HasSuffix(r.stderr, "(the package's C and C++ files: ok.cpp)\n") {
			t.Errorf("gocpp build with a missing function = %+v, want exit 1 and the link error", r)
		}
	})

	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_CC", "gocpp-no-such-cc")
	inDir(t, map[string]string{"main.gp": mainSrc, "a.c": "int a;\n"})
	if r := gocppRun("", false, "build", "."); r.code != 1 || r.stderr != "gocpp build: GOCPP_CC: the C compiler \"gocpp-no-such-cc\" was not found: exec: \"gocpp-no-such-cc\": executable file not found in $PATH\n" {
		t.Errorf("gocpp build with a missing GOCPP_CC = %+v", r)
	}
	// Without .c files, no C compiler is needed.
	os.Remove("a.c")
	if r := gocppRun("", true, "run", "."); r != (result{0, "main\n", ""}) {
		t.Errorf("gocpp run without .c files and a missing GOCPP_CC = %+v", r)
	}
}

// TestSourcesNixHardening checks that the user's C and C++ files get
// gocpp/nofortify.h: under a Nix compiler wrapper with hardening on, their
// compiles at -O0 don't warn about _FORTIFY_SOURCE.
func TestSourcesNixHardening(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, map[string]string{"main.gp": mainSrc, "plain.cpp": "#include <cstdio>\nint main() { std::puts(\"x\"); }\n"})
		t.Setenv("NIX_HARDENING_ENABLE", nixHardening)
		out, _ := c.cxx.Command(context.Background(), append(build.Flags(c.cxx, false), "-c", "plain.cpp", "-o", filepath.Join(dir, "plain.o"))...).CombinedOutput()
		if !strings.Contains(string(out), "_FORTIFY_SOURCE requires compiling with optimization") {
			t.Skipf("%s with NIX_HARDENING_ENABLE doesn't warn about _FORTIFY_SOURCE at -O0: not a Nix compiler wrapper", c.cxx.Path)
		}
		os.Remove("plain.cpp")
		os.Remove("plain.o")
		writeFiles(t, dir, map[string]string{"a.c": "#include <string.h>\nint c_len(void) { return (int)strlen(\"abc\"); }\n",
			"b.cpp": "#include <cstring>\nint cxx_len() { return (int)std::strlen(\"ab\"); }\n"})
		// The C compiler warns too, without the header.
		cc, err := build.FindCC(context.Background(), c.cxx)
		if err != nil {
			t.Fatal(err)
		}
		out, _ = cc.Command(context.Background(), append(build.SourceFlags(cc, false), "-c", "a.c", "-o", filepath.Join(dir, "a.o"))...).CombinedOutput()
		os.Remove("a.o")
		if !strings.Contains(string(out), "_FORTIFY_SOURCE requires compiling with optimization") {
			t.Errorf("%s with NIX_HARDENING_ENABLE doesn't warn about _FORTIFY_SOURCE at -O0, but %s does:\n%s", cc.Path, c.cxx.Path, out)
		}
		callSources(t, `extern "C" int c_len(); int cxx_len();`, "c_len() * 10 + cxx_len()")
		if r := gocppRun("", true, "run", "."); r != (result{0, "sources 32\nmain\n", ""}) {
			t.Errorf("gocpp run with hardening = %+v, want exit 0 and no warnings", r)
		}
	})
}

// TestSourcesASan checks that -asan builds the user's C and C++ files with
// the sanitizers: a heap overflow in the C file is caught (without the
// instrumentation of a.c, nothing would see it).
func TestSourcesASan(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"main.gp": mainSrc,
			"a.c": "#include <stdlib.h>\nint c_overflow(int i) {\n\tchar *p = malloc(4);\n\tp[i] = 1;\n\tint v = p[0];\n\tfree(p);\n\treturn v;\n}\n"})
		callSources(t, `extern "C" int c_overflow(int);`, "c_overflow(4)")
		r := gocppRun("", false, "run", "-asan", ".")
		if r.code != 1 || !strings.Contains(r.stderr, "ERROR: AddressSanitizer: heap-buffer-overflow") {
			t.Errorf("gocpp run -asan of a heap overflow in a C file = %+v, want exit 1 and an AddressSanitizer report", r)
		}
		r = gocppRun("", false, "run", "-asan", "-x", ".")
		workDir(t, r.stderr)
		if !regexp.MustCompile(`(?m)^\S+ -g -O0 -fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all .* -c -x c \S+/a\.c `).MatchString(r.stderr) {
			t.Errorf("gocpp run -asan -x doesn't compile a.c with the sanitizers:\n%s", r.stderr)
		}
	})
}

// TestSourcesMixedFamilies checks the warning of -asan with a GOCPP_CC of
// another family than the C++ compiler.
func TestSourcesMixedFamilies(t *testing.T) {
	// It needs g++ and clang++ (whose clang is the C compiler): a missing
	// one is a skip, unless GOCPP_REQUIRE_CXX names it (testCompilers fails
	// then).
	have := map[string]bool{}
	for _, c := range testCompilers(t) {
		if c.missing == "" {
			have[filepath.Base(c.name)] = true
		}
	}
	if !have["g++"] || !have["clang++"] {
		t.Skip("needs g++ and clang++")
	}
	if _, err := build.LookCXX(context.Background(), "clang"); err != nil {
		t.Fatalf("clang++ is there, but clang isn't: %v", err)
	}
	t.Setenv("GOCPP_CXX", "g++")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_CC", "clang")
	dir := inDir(t, map[string]string{"main.gp": mainSrc, "a.c": "int a;\n"})
	r := gocppRun("", false, "build", "-asan", "-o", filepath.Join(dir, "prog"), ".")
	if m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr); m != nil {
		os.RemoveAll(m[1])
	}
	if !strings.HasPrefix(r.stderr, "gocpp build: warning: -asan with a C compiler (clang, GOCPP_CC) of another family than the C++ compiler (g++) is not supported\n") {
		t.Errorf("gocpp build -asan with GOCPP_CC=clang and g++ = %+v, want the warning", r)
	}
	// Without -asan, no warning.
	if r := gocppRun("", false, "build", "-o", filepath.Join(dir, "prog"), "."); r != (result{0, "", ""}) {
		t.Errorf("gocpp build with GOCPP_CC=clang and g++ = %+v, want exit 0 and no output", r)
	}
}
