package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build/testlib"
)

// The tests of the cxx recipes of C and C++ libraries in gocpp build and run
// (Phase 06 step 6.1d), with the real compilers. The library is a module
// that the project replaces with a local folder.

// ageTree makes every file under dir an hour old, so that the object cache
// does not take them for files that change right now (its racy window).
func ageTree(t *testing.T, dir string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil {
			os.Chtimes(p, old, old)
		}
		return nil
	})
}

// cacheOn turns the object cache on (TestMain turns it off). The cache folder is
// the tests' shared one, so the runtime is not built again: a test that wants a
// miss gives its library a source text of its own.
func cacheOn(t *testing.T) {
	t.Helper()
	t.Setenv("GOCPP_NOOBJCACHE", "0")
}

// withOneCXX runs fn with one installed compiler: for the tests of rules that
// do not depend on the compiler (the recipe's text, the paths, the messages); the
// interop folders and the build tests run both.
func withOneCXX(t *testing.T, fn func(t *testing.T, c testCXX)) {
	t.Helper()
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	fn(t, c)
}

// libProject returns the files of a project that imports a header of the
// module example.com/clib (in clib/) and prints what main.gp's body says.
func libProject(recipe, body string, lib map[string]string) map[string]string {
	files := map[string]string{
		"gocpp.mod": "module example.com/app\n\nrequire example.com/clib v1.0.0\nreplace example.com/clib => ./clib\n\ncxx example.com/clib {\n" + recipe + "\n}\n",
		"main.gp":   "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/clib/include/clib.h\"\n)\n\nfunc main() {\n" + body + "\n}\n",
	}
	for name, src := range lib {
		files["clib/"+name] = src
	}
	return files
}

const clibH = `#pragma once
#ifdef __cplusplus
extern "C" {
#endif
int clib_scaled(int x);
int clib_square(int x);
#ifdef __cplusplus
}
#endif
`

const clibC = `#include "clib.h"
#ifndef CLIB_BUILD
#error the private define is missing
#endif
#ifndef CLIB_SCALE
#error the public define is missing
#endif
int clib_scaled(int x) { return x * CLIB_SCALE; }
int clib_square(int x) { return x * x; }
`

func TestRecipeBuildC(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		dir := inDir(t, libProject("\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=3\n\tprivate-define CLIB_BUILD=1",
			"\tfmt.Println(clib.clib_scaled(14), clib.clib_square(7))",
			map[string]string{"include/clib.h": clibH, "src/clib.c": clibC}))
		if r := gocppRun("", false, "check", "."); r != (result{0, "", ""}) {
			t.Fatalf("gocpp check = %+v", r)
		}
		r := gocppRun("", false, "run", "-x", ".")
		workDir(t, r.stderr)
		if r.code != 0 || r.stdout != "42 49\n" {
			t.Fatalf("gocpp run = %+v", r)
		}
		// -x: the library compile has the private and the public flags, the
		// program's compile only the public ones; the object is linked.
		clib := filepath.Join(dir, "clib")
		var libCompile, appCompile, link string
		for _, l := range strings.Split(r.stderr, "\n") {
			switch {
			case strings.Contains(l, " -c -x c "+filepath.Join(clib, "src", "clib.c")):
				libCompile = l
			case strings.Contains(l, " -x c++ ") && strings.Contains(l, "/gpkg/") && strings.Contains(l, " -c "):
				appCompile = l
			case strings.Contains(l, "$WORK/libobj/0-0-clib.c.o") && strings.Contains(l, " -o $WORK/app"):
				link = l
			}
		}
		for _, want := range []string{" -std=gnu17 ", " -w ", "-O0", "-I " + filepath.Join(clib, "include"), " -DCLIB_SCALE=3 ", " -DCLIB_BUILD=1 ", "-iquote " + clib} {
			if !strings.Contains(libCompile, want) {
				t.Errorf("the library compile doesn't have %q:\n%s", want, libCompile)
			}
		}
		if !strings.Contains(appCompile, "-I "+filepath.Join(clib, "include")+" -DCLIB_SCALE=3 ") || strings.Contains(appCompile, "CLIB_BUILD") {
			t.Errorf("the program's compile has the public flags only:\n%s", appCompile)
		}
		if link == "" {
			t.Errorf("-x shows no link with the library's object:\n%s", r.stderr)
		}
	})
}

const cxxH = `#pragma once
#include <libx/util.h>
#ifdef LAYOUT_V2
struct Pair { int a; int b; int extra; };
#else
struct Pair { int a; int b; };
#endif
inline int header_size() { return (int)sizeof(Pair); }
int lib_size();
int lib_mode();
inline int pair_sum(Pair p) { return util_add(p.a, p.b); }
`

const cxxCPP = `#include "cxx.hpp"
#if __cplusplus != 201703L
#error the library is C++17
#endif
int lib_size() { return (int)sizeof(Pair); }
int lib_mode() {
#ifdef MODE_FAST
	return 1;
#else
	return 0;
#endif
}
`

func TestRecipeBuildCXX(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		// A -D that changes the layout of a header must reach both sides:
		// header_size() is compiled into the program, lib_size() into the library.
		// The header includes <libx/util.h>, found through the include folder.
		body := "\tfmt.Println(cxx.header_size() == cxx.lib_size(), cxx.header_size(), cxx.lib_mode())"
		files := libProject("\tsrc cxx.cpp\n\tinclude include\n\tdefine LAYOUT_V2\n\tprivate-define MODE_FAST\n\tstd c++17",
			body, map[string]string{"include/cxx.hpp": cxxH, "include/libx/util.h": "#pragma once\ninline int util_add(int a, int b) { return a + b; }\n", "cxx.cpp": cxxCPP})
		files["main.gp"] = strings.Replace(files["main.gp"], "include/clib.h", "include/cxx.hpp", 1)
		files["main.gp"] = strings.Replace(files["main.gp"], "clib.h\"", "cxx.hpp\"", 1)
		inDir(t, files)
		if r := gocppRun("", false, "run", "."); r != (result{0, "true 12 1\n", ""}) {
			t.Fatalf("gocpp run = %+v", r)
		}
		// Without the public define, the two sides disagree: the define is
		// what the recipe's `define` does.
		files["gocpp.mod"] = strings.Replace(files["gocpp.mod"], "\tdefine LAYOUT_V2\n", "\tprivate-define LAYOUT_V2\n", 1)
		writeFiles(t, ".", files)
		if r := gocppRun("", false, "run", "."); r != (result{0, "false 8 1\n", ""}) {
			t.Fatalf("gocpp run with a private LAYOUT_V2 = %+v, want the sides to disagree", r)
		}
		// Without `include`, <libx/util.h> is not found (the module folder
		// is only an -iquote folder).
		files["gocpp.mod"] = strings.Replace(files["gocpp.mod"], "\tinclude include\n", "", 1)
		writeFiles(t, ".", files)
		if r := gocppRun("", false, "run", "."); r.code == 0 || !strings.Contains(r.stderr, "libx/util.h") {
			t.Fatalf("gocpp run without include = %+v", r)
		}
	})
}

func TestRecipeObjectCache(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		cacheOn(t)
		src := strings.Replace(clibC, "int clib_square", fmt.Sprintf("/* %s %d */\nint clib_square", c.name, time.Now().UnixNano()), 1)
		lib := map[string]string{"include/clib.h": clibH, "src/clib.c": src}
		recipe := "\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=3\n\tprivate-define CLIB_BUILD=1"
		body := "\tfmt.Println(clib.clib_scaled(14))"
		compile := regexp.MustCompile(`(?m)^\S+ .* -c -x c \S*/src/clib\.c -o `)
		hit := regexp.MustCompile(`(?m)^# cache hit clib\.c [0-9a-f]{16}$`)

		// Project 1 builds the library; project 2, another folder with another
		// gocpp.mod and the same library folder, reuses the object.
		libDir := t.TempDir()
		writeFiles(t, libDir, lib)
		ageTree(t, libDir)
		mk := func(recipe string) {
			files := libProject(recipe, body, nil)
			files["gocpp.mod"] = strings.Replace(files["gocpp.mod"], "./clib", libDir, 1)
			inDir(t, files)
			ageTree(t, ".")
		}
		build := func() result {
			r := gocppRun("", false, "build", "-x", "-o", "prog", ".")
			workDir(t, r.stderr)
			return r
		}
		mk(recipe)
		first := build()
		if first.code != 0 || !compile.MatchString(first.stderr) || hit.MatchString(first.stderr) {
			t.Fatalf("the first build doesn't compile the library: %+v", first)
		}
		second := build()
		if second.code != 0 || compile.MatchString(second.stderr) || !hit.MatchString(second.stderr) {
			t.Fatalf("the second build compiles the library again, or has no cache hit: %+v", second)
		}
		mk(recipe + "\n\t// another project")
		other := build()
		if other.code != 0 || compile.MatchString(other.stderr) || !hit.MatchString(other.stderr) {
			t.Fatalf("the second project compiles the library again: %+v", other)
		}
		// The misses for another flag, define, compiler, asan, module and version are
		// the unit test TestLibCacheMatrix (internal/build).
		// Another source text misses.
		lib["src/clib.c"] = strings.Replace(src, "x * x", "x * x + 0", 1)
		writeFiles(t, libDir, lib)
		ageTree(t, libDir)
		if r := build(); r.code != 0 || !compile.MatchString(r.stderr) || hit.MatchString(r.stderr) {
			t.Errorf("another source text is a hit: %+v", r)
		}
		// The cache off: the library compiles every time.
		t.Setenv("GOCPP_NOOBJCACHE", "1")
		if r := build(); r.code != 0 || !compile.MatchString(r.stderr) {
			t.Errorf("GOCPP_NOOBJCACHE=1: %+v", r)
		}
	})
}

// manyLib is a library of n C files, each with a function; sum.c calls them.
func manyLib(n int) (lib map[string]string, want int) {
	lib = map[string]string{}
	var decls, calls strings.Builder
	for i := 0; i < n; i++ {
		lib[fmt.Sprintf("src/f%02d.c", i)] = fmt.Sprintf("int f%02d(void) { return %d; }\n", i, i+1)
		fmt.Fprintf(&decls, "int f%02d(void);\n", i)
		fmt.Fprintf(&calls, "\tt += f%02d();\n", i)
		want += i + 1
	}
	lib["src/sum.c"] = decls.String() + "int many_sum(void) {\n\tint t = 0;\n" + calls.String() + "\treturn t;\n}\n"
	lib["include/many.h"] = "#pragma once\nint many_sum(void);\n"
	return lib, want
}

func TestRecipeParallelBuild(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		lib, want := manyLib(10)
		files := libProject("\tsrc src/*.c\n\tinclude include\n\tlink m", "\tfmt.Println(many.many_sum())", lib)
		files["main.gp"] = strings.Replace(files["main.gp"], "include/clib.h", "include/many.h", 1)
		files["main.gp"] = strings.Replace(files["main.gp"], "clib.h\"", "many.h\"", 1)
		dir := inDir(t, files)
		r := gocppRun("", false, "run", "-x", ".")
		workDir(t, r.stderr)
		if r.code != 0 || r.stdout != fmt.Sprintf("%d\n", want) {
			t.Fatalf("gocpp run = %+v", r)
		}
		if n := strings.Count(r.stderr, " -c -x c "+filepath.Join(dir, "clib", "src")); n != 11 {
			t.Errorf("%d library compiles, want 11", n)
		}
		// The link: the application's object, the library's objects (in order),
		// the runtime, and the library flags after them all.
		var link string
		for _, l := range strings.Split(r.stderr, "\n") {
			if strings.Contains(l, "$WORK/libobj/0-0-f00.c.o") && strings.Contains(l, " -o ") {
				link = l
			}
		}
		iApp, iLib0, iLib40, iRT, iM := strings.Index(link, ".gp.o"), strings.Index(link, "0-0-f00.c.o"), strings.Index(link, "0-10-sum.c.o"), strings.Index(link, "src_complex.o"), strings.LastIndex(link, " -lm ")
		if link == "" || iApp < 0 && !strings.Contains(link, "$WORK/app.o") || iLib0 < 0 || iLib40 < iLib0 || iRT < iLib40 || iM < iRT {
			t.Errorf("link order: objects of the library after the program's and before the runtime's, -lm last:\n%s", link)
		}
	})
}

func TestRecipeLibraryDoesNotCompile(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		dir := inDir(t, libProject("\tinclude include\n\tsrc src/ok.c\n\tsrc src/bad.c\n", "\tfmt.Println(clib.clib_square(2))",
			map[string]string{"include/clib.h": clibH, "src/ok.c": "int clib_square(int x) { return x * x; }\n",
				"src/bad.c": "int clib_scaled(int x) { return x * UNDEFINED_NAME; }\n"}))
		r := gocppRun("", false, "build", "-x", "-o", filepath.Join(dir, "prog"), ".")
		if r.code != 1 {
			t.Fatalf("gocpp build = %+v, want exit 1 (an error of the build, not an internal error)", r)
		}
		for _, want := range []string{
			"# example.com/clib\n",
			"the C or C++ library of module example.com/clib@v1.0.0 did not build: src/bad.c did not compile (recipe ./gocpp.mod:9: src src/bad.c)",
			"UNDEFINED_NAME",
		} {
			if !strings.Contains(r.stderr, want) {
				t.Errorf("stderr doesn't have %q:\n%s", want, r.stderr)
			}
		}
		if strings.Contains(r.stderr, "internal error") || strings.Contains(r.stderr, "bug in gocpp") {
			t.Errorf("a library that does not compile is not a bug of gocpp:\n%s", r.stderr)
		}
		// -x had the exact command.
		if !strings.Contains(r.stderr, " -c -x c "+filepath.Join(dir, "clib", "src", "bad.c")) {
			t.Errorf("-x doesn't show the command:\n%s", r.stderr)
		}
		// Without -x, the message says how to get it.
		r = gocppRun("", false, "build", "-o", filepath.Join(dir, "prog"), ".")
		if r.code != 1 || !strings.Contains(r.stderr, "(gocpp build -x shows the command)") || strings.Contains(r.stderr, "WORK=") {
			t.Errorf("gocpp build = %+v", r)
		}
		// A run: the same error, exit 1.
		if r := gocppRun("", false, "run", "."); r.code != 1 || !strings.Contains(r.stderr, "UNDEFINED_NAME") {
			t.Errorf("gocpp run = %+v", r)
		}
	})
}

func TestRecipeRecipeOfTheLibrary(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		// The library ships its recipe in its own gocpp.mod; the project has none.
		libMod := "module example.com/clib\n\ncxx example.com/clib {\n\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=5\n\tprivate-define CLIB_BUILD=1\n}\n"
		files := libProject("", "\tfmt.Println(clib.clib_scaled(2))", map[string]string{"include/clib.h": clibH, "src/clib.c": clibC, "gocpp.mod": libMod})
		files["gocpp.mod"] = "module example.com/app\n\nrequire example.com/clib v1.0.0\nreplace example.com/clib => ./clib\n"
		inDir(t, files)
		if r := gocppRun("", false, "run", "."); r != (result{0, "10\n", ""}) {
			t.Fatalf("the library's recipe: gocpp run = %+v", r)
		}
		// The project's block overrides it entirely (no merging): a different
		// scale, and the library's `private-define` is gone, so the library's
		// #error fires: the project has to say it again.
		files["gocpp.mod"] += "\ncxx example.com/clib {\n\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=7\n}\n"
		writeFiles(t, ".", files)
		if r := gocppRun("", false, "run", "."); r.code != 1 || !strings.Contains(r.stderr, "the private define is missing") {
			t.Fatalf("the project's recipe doesn't replace the library's: gocpp run = %+v", r)
		}
		files["gocpp.mod"] = strings.Replace(files["gocpp.mod"], "define CLIB_SCALE=7", "define CLIB_SCALE=7\n\tprivate-define CLIB_BUILD=1", 1)
		writeFiles(t, ".", files)
		if r := gocppRun("", false, "run", "."); r != (result{0, "14\n", ""}) {
			t.Fatalf("the project's recipe: gocpp run = %+v", r)
		}
	})
}

func TestRecipeVendored(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		files := libProject("\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=3\n\tprivate-define CLIB_BUILD=1", "\tfmt.Println(clib.clib_scaled(3))", nil)
		files["gocpp.mod"] = strings.Replace(files["gocpp.mod"], "replace example.com/clib => ./clib\n", "", 1)
		files["vendor/modules.txt"] = "# example.com/clib v1.0.0\n## explicit\nexample.com/clib\n"
		files["vendor/example.com/clib/include/clib.h"] = clibH
		files["vendor/example.com/clib/src/clib.c"] = clibC
		inDir(t, files)
		if r := gocppRun("", false, "run", "."); r != (result{0, "9\n", ""}) {
			t.Fatalf("a vendored library: gocpp run = %+v", r)
		}
	})
}

func TestRecipeErrorsAtTheLine(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		lib := map[string]string{"include/clib.h": clibH, "src/clib.c": clibC}
		for _, tc := range []struct{ recipe, want string }{
			// check finds the text and flag errors, build finds the files too.
			{"\tsrc src/clib.c\n\tfrobnicate 1", "./gocpp.mod:8: cxx example.com/clib: unknown statement \"frobnicate\""},
			{"\tsrc src/clib.c\n\tcflags -fplugin=evil.so", "./gocpp.mod:8: cxx example.com/clib: cflags: invalid flag in the cxx recipe of example.com/clib: -fplugin=evil.so"},
			{"\tsrc src/clib.c ../escape.c", `./gocpp.mod:7: cxx example.com/clib: src "../escape.c": a ".." element`},
		} {
			inDir(t, libProject(tc.recipe, "\tfmt.Println(clib.clib_square(2))", lib))
			for _, cmd := range []string{"check", "build", "run"} {
				r := gocppRun("", false, cmd, ".")
				if r.code != 1 || !strings.Contains(r.stderr, tc.want) {
					t.Errorf("gocpp %s with %q = %+v, want exit 1 and %q", cmd, tc.recipe, r, tc.want)
				}
			}
		}
		// The files are looked at by build and run.
		inDir(t, libProject("\tinclude include\n\tsrc src/clib.c\n\tsrc src/missing.c\n\tsrc src/*.cc", "\tfmt.Println(clib.clib_square(2))", lib))
		if r := gocppRun("", false, "check", "."); r.code != 0 {
			t.Errorf("gocpp check needs no sources: %+v", r)
		}
		for _, cmd := range []string{"build", "run"} {
			r := gocppRun("", false, cmd, ".")
			want := "# example.com/app\n./gocpp.mod:9: cxx example.com/clib: src src/missing.c: no such file in the module\n./gocpp.mod:10: cxx example.com/clib: src src/*.cc: the pattern matches no C or C++ source file\n"
			if r.code != 1 || r.stderr != want {
				t.Errorf("gocpp %s = exit %d, stderr:\n%swant\n%s", cmd, r.code, r.stderr, want)
			}
		}
	})
}

func TestRecipeSymlinkOut(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		outside := t.TempDir()
		os.WriteFile(filepath.Join(outside, "evil.c"), []byte("int clib_square(int x) { return 0; }\n"), 0o644)
		files := libProject("\tinclude include\n\tsrc src/*.c", "\tfmt.Println(clib.clib_square(2))", map[string]string{"include/clib.h": clibH, "src/real.c": "int clib_scaled(int x) { return x; }\n"})
		dir := inDir(t, files)
		if err := os.Symlink(filepath.Join(outside, "evil.c"), filepath.Join(dir, "clib", "src", "evil.c")); err != nil {
			t.Skip("no symbolic links:", err)
		}
		for _, cmd := range []string{"build", "run"} {
			r := gocppRun("", false, cmd, ".")
			if r.code != 1 || !strings.Contains(r.stderr, "a symbolic link leaves the module") || !strings.Contains(r.stderr, "./gocpp.mod:8: cxx example.com/clib: src src/*.c: src/evil.c: ") {
				t.Errorf("gocpp %s = %+v", cmd, r)
			}
		}
	})
}

func TestRecipeNoRecipeLinkError(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		// A header import of a module whose .c is not built: the link error
		// says that the module has no cxx recipe.
		files := libProject("", "\tfmt.Println(clib.clib_square(2))", map[string]string{"include/clib.h": clibH, "src/clib.c": clibC})
		files["gocpp.mod"] = "module example.com/app\n\nrequire example.com/clib v1.0.0\nreplace example.com/clib => ./clib\n"
		inDir(t, files)
		r := gocppRun("", false, "build", ".")
		if r.code != 1 || !strings.Contains(r.stderr, "(module example.com/clib has no cxx recipe, so its C and C++ files are not built: add to gocpp.mod  cxx example.com/clib { src file.c ... }") {
			t.Errorf("gocpp build = %+v", r)
		}
		// A header-only module (no C or C++ file in its folder) that the program
		// calls without a definition: the link fails, and the hint would be wrong.
		files = libProject("", "\tfmt.Println(clib.clib_square(2))", map[string]string{"include/clib.h": clibH})
		files["gocpp.mod"] = "module example.com/app\n\nrequire example.com/clib v1.0.0\nreplace example.com/clib => ./clib\n"
		inDir(t, files)
		r = gocppRun("", false, "build", ".")
		if r.code != 1 || !strings.Contains(r.stderr, "clib_square") || strings.Contains(r.stderr, "has no cxx recipe") {
			t.Errorf("gocpp build of a header-only module = %+v", r)
		}
		// A link that fails for another reason (a library that is not there): no hint either.
		files = libProject("", "\tfmt.Println(clib.clib_square(2))", map[string]string{"include/clib.h": clibH, "src/clib.c": clibC})
		files["gocpp.mod"] = "module example.com/app\n\nrequire example.com/clib v1.0.0\nreplace example.com/clib => ./clib\n"
		files["main.gp"] = "//gocpp:link gocpp_no_such_library\n" + files["main.gp"]
		inDir(t, files)
		r = gocppRun("", false, "build", ".")
		if r.code != 1 || strings.Contains(r.stderr, "has no cxx recipe") || !strings.Contains(r.stderr, "gocpp_no_such_library") {
			t.Errorf("gocpp build with a missing library = %+v", r)
		}
	})
}

func TestRecipeIncludeShadowsStdHeader(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		// `include .` with a stdint.h at the root of the library: the library's file
		// hides the system's in every compile with -I. The error says why.
		lib := map[string]string{"include/clib.h": "#pragma once\n#include <stdint.h>\nint clib_scaled(int x);\nint clib_square(int x);\n",
			"src/clib.c": clibC, "stdint.h": "/* not the system's */\n"}
		dir := inDir(t, libProject("\tsrc src/clib.c\n\tinclude .\n\tdefine CLIB_SCALE=3\n\tprivate-define CLIB_BUILD=1", "\tfmt.Println(clib.clib_square(2))", lib))
		for _, cmd := range []string{"build", "run"} {
			r := gocppRun("", false, cmd, ".")
			want := "the recipe's include folder " + filepath.Join(dir, "clib") + " has a file named stdint.h, which hides the standard header of that name"
			if r.code == 0 || !strings.Contains(r.stderr, want) {
				t.Errorf("gocpp %s = %+v, want a hint %q", cmd, r, want)
			}
		}
	})
}

func TestRecipeStrayRecipeWarning(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		files := libProject("\tsrc src/clib.c\n\tinclude include\n\tdefine CLIB_SCALE=3\n\tprivate-define CLIB_BUILD=1", "\tfmt.Println(clib.clib_square(2))",
			map[string]string{"include/clib.h": clibH, "src/clib.c": clibC})
		// A typo in the module path of a second block, and blocks for the project's own and a replaced module.
		files["gocpp.mod"] += "\ncxx example.com/clibb {\n\tsrc x.c\n}\ncxx example.com/app {\n}\n"
		inDir(t, files)
		want := "./gocpp.mod:13: warning: cxx example.com/clibb: this module is not the main module and is neither required nor replaced here, so the recipe is never used"
		r := gocppRun("", false, "check", ".")
		if r.code != 0 || r.stdout != "" || strings.Count(r.stderr, "warning:") != 1 || !strings.Contains(r.stderr, want) {
			t.Errorf("gocpp check = %+v, want exit 0 and the warning %q", r, want)
		}
		r = gocppRun("", false, "run", ".")
		if r.code != 0 || r.stdout != "4\n" || !strings.Contains(r.stderr, want) {
			t.Errorf("gocpp run = %+v", r)
		}
	})
}

func TestRecipePkgConfig(t *testing.T) {
	testlib.NeedPkgConfig(t)
	withOneCXX(t, func(t *testing.T, c testCXX) {
		prefix := filepath.Clean(t.TempDir())
		pcDir := testlib.Install(t, prefix, &c.cxx)
		t.Setenv("PKG_CONFIG_PATH", pcDir)
		t.Setenv("PKG_CONFIG", "")
		t.Setenv("GOCPP_GATE_PHASE", "04")
		// The library's source needs gptestlib's flags, and so does the link of
		// the program; a .pc file inside the module is never read.
		inDir(t, libProject("\tsrc src/pc.c\n\tinclude include\n\tpkg-config gptestlib",
			"\tfmt.Println(clib.clib_scaled(1))",
			map[string]string{"include/clib.h": clibH, "src/pc.c": "#include <gptestlib.h>\nint clib_scaled(int x) { return x + gptestlib_answer(); }\nint clib_square(int x) { return x; }\n",
				"gptestlib.pc": "Name: x\nDescription: x\nVersion: 1\nCflags: -DEVIL\n"}))
		if r := gocppRun("", false, "run", "."); r != (result{0, "43\n", ""}) {
			t.Fatalf("gocpp run = %+v", r)
		}
		// A package that does not exist is an error at the line.
		writeFiles(t, ".", libProject("\tsrc src/pc.c\n\tinclude include\n\tpkg-config nonexistent-gocpp-package", "\tfmt.Println(1)", nil))
		if r := gocppRun("", false, "check", "."); r.code != 1 || !strings.Contains(r.stderr, "./gocpp.mod:9: cxx example.com/clib: pkg-config: ") {
			t.Errorf("gocpp check = %+v", r)
		}
	})
}
