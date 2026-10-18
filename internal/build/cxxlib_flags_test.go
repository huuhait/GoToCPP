package build_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The flags of the cxx recipes (Phase 06 step 6.1d): the include folders and
// defines that reach the package, the allow-list for the flags of a recipe,
// the paths that must stay in the module, and who supplies the recipe.

// loadTree writes files (slash paths) under a new folder, loads the package
// of the folder main (a .gp file that imports the header of the module) and
// returns it. Errors of the load fail the test.
func loadTree(t *testing.T, files map[string]string) (lp *load.Package, prog *load.Program, dir string) {
	t.Helper()
	dir = filepath.Clean(t.TempDir())
	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog = load.Load(load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	lp = prog.Roots[0]
	if lp.Errors.HasErrors() {
		t.Fatalf("load errors: %v", lp.Errors.Diagnostics())
	}
	return lp, prog, dir
}

const recipeMain = "package main\n\nimport \"example.com/lib/lib.h\"\n\nfunc main() { lib.lib_f() }\n"

// recipeProject is a project with the module example.com/lib in lib/, replaced,
// with the recipe lines in the project's gocpp.mod.
func recipeProject(recipe ...string) map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/app\n\nrequire example.com/lib v1.2.3\nreplace example.com/lib => ./lib\n\ncxx example.com/lib {\n" +
			strings.Join(recipe, "\n") + "\n}\n",
		"main.gp":       recipeMain,
		"lib/lib.h":     "void lib_f(void);\n",
		"lib/lib.c":     "void lib_f(void) {}\n",
		"lib/inc/a.h":   "\n",
		"lib/sys/b.h":   "\n",
		"lib/more/c.h":  "\n",
		"lib/inc/x.h":   "\n",
		"lib/sub/d.cpp": "\n",
	}
}

func readFlags(t *testing.T, lp *load.Package, prog *load.Program, pc build.PkgConfigFunc) (build.PackageFlags, []diag.Diagnostic) {
	t.Helper()
	flags, errs, pending := build.ReadPackageFlags(context.Background(), prog.Fset, lp, pc)
	if pending {
		t.Fatal("pending")
	}
	return flags, errs
}

func TestRecipeFlagsOrder(t *testing.T) {
	files := recipeProject(
		"\tsrc lib.c sub/d.cpp",
		"\tinclude inc more",
		"\tsystem-include sys",
		"\tdefine PUB=1 PUBV=\"x y\"",
		"\tprivate-define PRIV=2",
		"\tcflags -O2 -DC1",
		"\tcxxflags -fno-rtti",
		"\tstd c++17",
		"\tcstd c11",
		"\tlink m -lpthread",
		"\tldflags -Linc",
		"\tpkg-config zlib",
		"\tlinux: define ON_LINUX",
		"\twindows: define ON_WINDOWS",
		"\t!linux: define NOT_LINUX",
	)
	files["main.gp"] = "//gocpp:cflags -DDIRECTIVE\n//gocpp:link rt\n" + recipeMain
	lp, prog, dir := loadTree(t, files)
	pc := &fakePkgConfig{flags: map[string][2]string{"zlib": {"-DFROM_PC -I/usr/include/zlib", "-lz"}}}
	flags, errs := readFlags(t, lp, prog, pc.run)
	if len(errs) > 0 {
		t.Fatalf("errors: %s", errText(errs))
	}
	lib := filepath.Join(dir, "lib")
	wantCompile := []string{
		"-DDIRECTIVE", // the package's own flags first
		"-I", filepath.Join(lib, "inc"), "-I", filepath.Join(lib, "more"),
		"-isystem", filepath.Join(lib, "sys"),
		"-DPUB=1", "-DPUBV=x y",
		"-DFROM_PC", "-I/usr/include/zlib",
		"-DON_LINUX",
		"-iquote", lib, // the module folder last
	}
	if !slices.Equal(flags.Compile, wantCompile) {
		t.Errorf("Compile =\n%q\nwant\n%q", flags.Compile, wantCompile)
	}
	if flags.Directive != 1 {
		t.Errorf("Directive = %d", flags.Directive)
	}
	if wantLink := []string{"-lrt", "-lm", "-lpthread", "-L" + filepath.Join(lib, "inc"), "-lz"}; !slices.Equal(flags.Link, wantLink) {
		t.Errorf("Link = %q, want %q", flags.Link, wantLink)
	}
	if len(flags.Libs) != 1 {
		t.Fatalf("Libs = %v", flags.Libs)
	}
	l := flags.Libs[0]
	if l.Module != "example.com/lib" || l.Version != "v1.2.3" || l.Dir != lib || filepath.Base(l.File) != "gocpp.mod" || l.FromLibrary {
		t.Errorf("lib = %+v", l)
	}
	if !slices.Equal(l.Private, []string{"-DPRIV=2", "-O2", "-DC1"}) || !slices.Equal(l.PrivateCXX, []string{"-fno-rtti"}) {
		t.Errorf("private flags = %q, %q", l.Private, l.PrivateCXX)
	}
	if l.Std != "c++17" || l.CStd != "c11" {
		t.Errorf("standards = %q %q", l.Std, l.CStd)
	}
	if !slices.Equal(l.Public, wantCompile[1:len(wantCompile)-2]) {
		t.Errorf("Public = %q", l.Public)
	}
	if srcs, serrs := l.Sources(); len(serrs) > 0 || !slices.Equal(sourceRels(srcs), []string{"lib.c", "sub/d.cpp"}) {
		t.Errorf("sources = %v, %v", srcs, serrs)
	}
	// The probe flags have the recipe's, in the same place as the compile's.
	probe := build.ProbeFlags(build.EnvFlags{}, flags)
	if !strings.Contains(strings.Join(probe, " "), "-DPUB=1") {
		t.Errorf("ProbeFlags = %q", probe)
	}
	// The other OS: the conditions choose.
	lp.CxxModules[0].GOOS = "windows"
	flags, errs = readFlags(t, lp, prog, pc.run)
	if len(errs) > 0 || !slices.Contains(flags.Compile, "-DON_WINDOWS") || !slices.Contains(flags.Compile, "-DNOT_LINUX") || slices.Contains(flags.Compile, "-DON_LINUX") {
		t.Errorf("windows: %q %s", flags.Compile, errText(errs))
	}
}

func sourceRels(srcs []build.LibSource) []string {
	var out []string
	for _, s := range srcs {
		out = append(out, s.Rel)
	}
	return out
}

func TestRecipeNone(t *testing.T) {
	// No recipe anywhere: header-only use, as before 6.1d.
	files := recipeProject()
	files["gocpp.mod"] = "module example.com/app\n\nrequire example.com/lib v1.2.3\nreplace example.com/lib => ./lib\n"
	lp, prog, dir := loadTree(t, files)
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 || len(flags.Libs) != 0 || !slices.Equal(flags.Compile, []string{"-iquote", filepath.Join(dir, "lib")}) {
		t.Errorf("flags = %+v, errors %s", flags, errText(errs))
	}
	if len(lp.CxxModules) != 1 || lp.CxxModules[0].Recipe != nil {
		t.Errorf("CxxModules = %+v", lp.CxxModules)
	}
}

func TestRecipeFromLibraryAndOverride(t *testing.T) {
	libMod := "module example.com/lib\ncxx c++17\n\ncxx example.com/lib {\n\tsrc lib.c\n\tinclude inc\n\tdefine FROM_LIB\n}\n"
	files := recipeProject()
	files["gocpp.mod"] = "module example.com/app\n\nrequire example.com/lib v1.2.3\nreplace example.com/lib => ./lib\n"
	files["lib/gocpp.mod"] = libMod
	lp, prog, dir := loadTree(t, files)
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 {
		t.Fatal(errText(errs))
	}
	lib := filepath.Join(dir, "lib")
	if !slices.Equal(flags.Compile, []string{"-I", filepath.Join(lib, "inc"), "-DFROM_LIB", "-iquote", lib}) {
		t.Errorf("the library's own recipe: Compile = %q", flags.Compile)
	}
	if l := flags.Libs[0]; !l.FromLibrary || l.Std != "c++17" || filepath.Dir(l.File) != lib {
		t.Errorf("lib = %+v (its own file's cxx line is the default standard)", l)
	}

	// The project's block replaces the library's entirely: no merging.
	files["gocpp.mod"] += "\ncxx example.com/lib {\n\tsrc sub/d.cpp\n\tdefine FROM_PROJECT\n}\n"
	lp, prog, dir = loadTree(t, files)
	flags, errs = readFlags(t, lp, prog, nil)
	if len(errs) > 0 {
		t.Fatal(errText(errs))
	}
	if !slices.Equal(flags.Compile, []string{"-DFROM_PROJECT", "-iquote", filepath.Join(dir, "lib")}) {
		t.Errorf("the project's recipe: Compile = %q", flags.Compile)
	}
	l := flags.Libs[0]
	if l.FromLibrary || filepath.Dir(l.File) != dir || l.Std != "c++17" {
		t.Errorf("lib = %+v", l)
	}
	if srcs, _ := l.Sources(); !slices.Equal(sourceRels(srcs), []string{"sub/d.cpp"}) {
		t.Errorf("sources = %v: the library's `src lib.c` must not be merged", sourceRels(srcs))
	}
}

func TestRecipeOwnModule(t *testing.T) {
	// A project's recipe for its own module (native C code of the project).
	files := map[string]string{
		"gocpp.mod":    "module example.com/app\n\ncxx example.com/app {\n\tsrc native/n.c\n\tinclude native\n\tdefine OWN=1\n}\n",
		"main.gp":      "package main\n\nimport \"example.com/app/native/n.h\"\n\nfunc main() { n.n_f() }\n",
		"native/n.h":   "void n_f(void);\n",
		"native/n.c":   "void n_f(void) {}\n",
		"native/o.cpp": "\n",
	}
	lp, prog, dir := loadTree(t, files)
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 {
		t.Fatal(errText(errs))
	}
	if !slices.Equal(flags.Compile, []string{"-I", filepath.Join(dir, "native"), "-DOWN=1", "-iquote", dir}) || len(flags.Libs) != 1 {
		t.Errorf("flags = %+v", flags)
	}
}

func TestRecipeVendoredModule(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":                           "module example.com/app\n\nrequire example.com/lib v1.2.3\n\ncxx example.com/lib {\n\tsrc lib.c\n\tinclude inc\n}\n",
		"main.gp":                             recipeMain,
		"vendor/modules.txt":                  "# example.com/lib v1.2.3\n## explicit\nexample.com/lib\n",
		"vendor/example.com/lib/lib.h":        "void lib_f(void);\n",
		"vendor/example.com/lib/lib.c":        "void lib_f(void) {}\n",
		"vendor/example.com/lib/inc/a.h":      "\n",
		"vendor/example.com/lib/gocpp.mod":    "module example.com/lib\ncxx example.com/lib {\n\tsrc nothing.c\n}\n",
		"vendor/example.com/lib/nothing.c.h":  "\n",
		"vendor/example.com/lib/inc/other.cc": "\n",
	}
	lp, prog, dir := loadTree(t, files)
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 {
		t.Fatal(errText(errs))
	}
	lib := filepath.Join(dir, "vendor", "example.com", "lib")
	if !slices.Equal(flags.Compile, []string{"-I", filepath.Join(lib, "inc"), "-iquote", lib}) || flags.Libs[0].FromLibrary {
		t.Errorf("flags = %q libs %+v", flags.Compile, flags.Libs[0])
	}
}

// TestRecipeErrors checks the errors of a recipe at its line, and that a
// recipe with an error adds nothing.
func TestRecipeErrors(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "evil.h"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ line, want string }{
		// The allow-list (Phase 04 step 4.15): the same refusals as for //gocpp:cflags.
		{"cflags -fplugin=evil.so", "invalid flag in the cxx recipe of example.com/lib: -fplugin=evil.so"},
		{"cflags -Xclang -load -Xclang evil.so", "invalid flag in the cxx recipe of example.com/lib: -Xclang"},
		{"cxxflags -wrapper evil.sh", "invalid flag in the cxx recipe of example.com/lib: -wrapper"},
		{"cflags -B/tmp/evil", "invalid flag in the cxx recipe of example.com/lib: -B/tmp/evil"},
		{"cflags -specs=evil.specs", "-specs=evil.specs"},
		{"cflags --sysroot=/tmp/x", "--sysroot=/tmp/x"},
		{"cflags @args.txt", "invalid flag in the cxx recipe of example.com/lib: @args.txt"},
		{"cflags -Wp,-include,/etc/passwd", "-Wp,-include,/etc/passwd"},
		{"cflags -Wa,-a=x.lst", "-Wa,-a=x.lst"},
		{"cflags -fuse-ld=gold", "a package can't choose the linker"},
		{"cflags -m32", "it changes the ABI"},
		{"define _GLIBCXX_DEBUG", "it changes the ABI"},
		{"private-define _GLIBCXX_USE_CXX11_ABI=0", "it changes the ABI"},
		{"define X=@file", "-DX=@file"},
		{"define X=a-b", "-DX=a-b"},
		{"ldflags -fuse-ld=gold", "a package can't choose the linker"},
		{"ldflags -lto_library=evil.so", "invalid flag"},
		{"ldflags -lto_library evil.so", "invalid flag"},
		{"ldflags @link.txt", "invalid flag in the cxx recipe of example.com/lib: @link.txt"},
		{"ldflags -Wl,--wrap=main,evil", "invalid flag"},
		{"ldflags -Wl,-rpath,/tmp/evil", "run-time library path"},
		{"ldflags -Wl,-R/tmp/evil", "run-time library path"},
		{"ldflags -Wl,--just-symbols=evil.o", "run-time library path"},
		{"ldflags -Wl,-sectcreate,a,b,/etc/passwd", "run-time library path"},
		{"ldflags -Wl,--push-state,--as-needed", "run-time library path"},
		{"ldflags -l:../../../../../../etc/passwd", "name a library by a path"},
		{"ldflags -l:../x", "name a library by a path"},
		{"ldflags -l../x", "name a library by a path"},
		{"ldflags -l:/etc/passwd", "name a library by a path"},
		{"ldflags -l:sub/libfoo.a", "name a library by a path"},
		{"ldflags -l ../../x", "name a library by a path"},
		{"ldflags -l /etc/x", "name a library by a path"},
		{"ldflags -Wl,-l:../x", "invalid flag"},
		{"ldflags -Wl,--library=../x", "invalid flag"},
		{"ldflags -Wl,-L/tmp", "invalid flag"},
		{"ldflags -Wl,-framework,Evil", "run-time library path"},
		{"link -l:libfoo.a", "want a library name"}, // the parser: `link` names libraries, not files
		{"link -fplugin=x", "want a library name"},  // the parser
		// Paths that leave the module.
		{"cflags -I/usr/include", "is not inside the module"},
		{"cflags -I ../outside", "is not inside the module"},
		{"cflags -I../outside", "is not inside the module"},
		{"cflags -isystem /usr/include", "is not inside the module"},
		{"cflags -include /etc/passwd", "is not inside the module"},
		{"cflags -include ../evil.h", "is not inside the module"},
		{"cflags -I=sysroot", "is not inside the module"},
		{"cflags -I$SYSROOT/x", "is not inside the module"},
		{"cflags -Isub/../../x", "is not inside the module"},
		{"cflags -Ileak", "a symbolic link leaves the module"},
		{"cflags -include leak/evil.h", "a symbolic link leaves the module"},
		{"cflags -F/System", "is not inside the module"},
		{"ldflags -L/usr/lib", "is not inside the module"},
		{"ldflags -L ../x", "is not inside the module"},
		{"ldflags /usr/lib/libfoo.a", "is not inside the module"},
		{"ldflags ../libfoo.a", "invalid flag in the cxx recipe of example.com/lib: ../libfoo.a"},
		{"ldflags ./../libfoo.a", "is not inside the module"},
		{"ldflags leak/libfoo.a", "a symbolic link leaves the module"},
		{"ldflags -Lleak", "a symbolic link leaves the module"},
		{"include leak", "a symbolic link leaves the module"},
		{"system-include leak", "a symbolic link leaves the module"},
		{"include nodir", "no such folder in the module"},
		{"include lib.h", "not a folder"},
		{"include ../x", `a ".." element`},
		{"system-include /usr/include", "an absolute path"},
		// Statements that gocpp does not know.
		{"newfeature x", `unknown statement "newfeature"`},
	} {
		files := recipeProject("\tsrc lib.c", "\t"+tc.line, "\tdefine AFTER")
		lp, prog, dir := loadTree(t, files)
		if err := os.Symlink(outside, filepath.Join(dir, "lib", "leak")); err != nil {
			t.Skip("no symbolic links:", err)
		}
		flags, errs := readFlags(t, lp, prog, nil)
		got := errText(errs)
		if len(errs) != 1 || !strings.Contains(got, tc.want) || !strings.HasPrefix(got, "gocpp.mod:8:0: cxx example.com/lib: ") {
			t.Errorf("%q: errors\n%swant one with %q at gocpp.mod line 8", tc.line, got, tc.want)
		}
		if len(flags.Libs) != 0 || slices.Contains(flags.Compile, "-DAFTER") || len(flags.Link) != 0 {
			t.Errorf("%q: a recipe with an error added flags: %+v", tc.line, flags)
		}
	}
}

// TestRecipeAllowListEnvAllow checks that GOCPP_CFLAGS_ALLOW (the user's
// choice) allows a flag in a recipe as it does in a directive.
func TestRecipeAllowListEnvAllow(t *testing.T) {
	t.Setenv("GOCPP_CFLAGS_ALLOW", "-fplugin=ok.so")
	lp, prog, _ := loadTree(t, recipeProject("\tcflags -fplugin=ok.so"))
	if _, errs := readFlags(t, lp, prog, nil); len(errs) > 0 {
		t.Errorf("errors: %s", errText(errs))
	}
	t.Setenv("GOCPP_CFLAGS_DISALLOW", "-DNO")
	lp, prog, _ = loadTree(t, recipeProject("\tdefine NO"))
	if _, errs := readFlags(t, lp, prog, nil); len(errs) != 1 {
		t.Errorf("GOCPP_CFLAGS_DISALLOW: errors: %s", errText(errs))
	}
}

func TestRecipePaths(t *testing.T) {
	// Paths in the flags are made absolute from the module's folder and
	// kept when they are inside.
	lp, prog, dir := loadTree(t, recipeProject("\tcflags -Iinc -I more -isystem sys -include inc/a.h -include missing.h -fPIC -Dx -D y", "\tldflags -Linc -L more sub/libx.a -lfoo -l:libbar.a -l baz"))
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 {
		t.Fatal(errText(errs))
	}
	lib := filepath.Join(dir, "lib")
	l := flags.Libs[0]
	wantPriv := []string{"-I" + filepath.Join(lib, "inc"), "-I", filepath.Join(lib, "more"), "-isystem", filepath.Join(lib, "sys"),
		"-include", filepath.Join(lib, "inc", "a.h"), "-include", "missing.h", "-fPIC", "-Dx", "-D", "y"}
	if !slices.Equal(l.Private, wantPriv) {
		t.Errorf("cflags = %q, want %q", l.Private, wantPriv)
	}
	wantLink := []string{"-L" + filepath.Join(lib, "inc"), "-L", filepath.Join(lib, "more"), filepath.Join(lib, "sub", "libx.a"), "-lfoo", "-l:libbar.a", "-l", "baz"}
	if !slices.Equal(l.Link, wantLink) {
		t.Errorf("ldflags = %q, want %q", l.Link, wantLink)
	}
}

func TestRecipePkgConfigHostileOutput(t *testing.T) {
	// The output of pkg-config for a recipe goes through the allow-list as
	// it does for a directive.
	lp, prog, _ := loadTree(t, recipeProject("\tpkg-config evil"))
	pc := &fakePkgConfig{flags: map[string][2]string{"evil": {"-fplugin=x.so", ""}}}
	if _, errs := readFlags(t, lp, prog, pc.run); len(errs) != 1 || !strings.Contains(errText(errs), "invalid flag in pkg-config --cflags: -fplugin=x.so") {
		t.Errorf("errors: %s", errText(errs))
	}
	pc = &fakePkgConfig{flags: map[string][2]string{"evil": {"", "-Wl,--wrap=a,b"}}}
	if _, errs := readFlags(t, lp, prog, pc.run); len(errs) != 1 || !strings.Contains(errText(errs), "pkg-config --libs") {
		t.Errorf("errors: %s", errText(errs))
	}
	pc = &fakePkgConfig{flags: map[string][2]string{}}
	_, errs := readFlags(t, lp, prog, pc.run)
	if len(errs) != 1 || !strings.Contains(errText(errs), "Package evil was not found") || !strings.HasPrefix(errText(errs), "gocpp.mod:7:0: cxx example.com/lib: pkg-config: ") {
		t.Errorf("a missing package: %s", errText(errs))
	}
	// pkg-config runs in the package's folder with names only.
	if len(pc.runs) != 1 || !slices.Equal(pc.runs[0], []string{"evil"}) {
		t.Errorf("runs = %v", pc.runs)
	}
}

func TestRecipePending(t *testing.T) {
	// The language server's pkg-config answers later: not an error.
	lp, prog, _ := loadTree(t, recipeProject("\tpkg-config zlib", "\tdefine A"))
	flags, errs, pending := build.ReadPackageFlags(context.Background(), prog.Fset, lp, func(context.Context, string, []string) ([]string, []string, error) {
		return nil, nil, build.ErrPending
	})
	if len(errs) > 0 || !pending || len(flags.Libs) != 0 {
		t.Errorf("pending = %v, errors %s, libs %v", pending, errText(errs), flags.Libs)
	}
}

func TestRecipeDuplicateBlocks(t *testing.T) {
	files := recipeProject("\tdefine A")
	files["gocpp.mod"] += "\ncxx example.com/lib {\n\tdefine B\n}\n"
	lp, prog, _ := loadTree(t, files)
	_, errs := readFlags(t, lp, prog, nil)
	if got := errText(errs); !strings.Contains(got, "gocpp.mod:10:0: multiple cxx recipes for example.com/lib (the first is at line 6)") {
		t.Errorf("errors: %s", got)
	}
}

func TestRecipeNoModuleHeaderNoRead(t *testing.T) {
	// A recipe of a module that the package does not import a header from is
	// never read (even a broken one).
	files := recipeProject("\tfrobnicate")
	files["main.gp"] = "package main\n\nfunc main() {}\n"
	lp, prog, dir := loadTree(t, files)
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 || len(flags.Libs) != 0 || len(flags.Compile) != 0 || len(lp.CxxModules) != 0 {
		t.Errorf("flags %+v errors %s modules %v in %s", flags, errText(errs), lp.CxxModules, dir)
	}
}

func ExampleReadPackageFlags_recipe() {
	fmt.Println("see TestRecipeFlagsOrder")
	// Output: see TestRecipeFlagsOrder
}
