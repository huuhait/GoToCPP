package build

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// The sources of a library (Phase 06 step 6.1d): patterns, exclude, the
// symbolic link check and the bounds on what a pattern looks at.

// libOf returns the library of the module folder dir with the recipe lines,
// as ReadPackageFlags makes it (only the src and exclude statements).
func libOf(t *testing.T, dir string, lines ...string) *CxxLib {
	t.Helper()
	src := "module example.com/app\ncxx example.com/lib {\n" + strings.Join(lines, "\n") + "\n}\n"
	f, errs := gocppmod.Parse("gocpp.mod", []byte(src), nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	blocks, _ := f.Recipes()
	r, rerrs := gocppmod.ParseRecipe("gocpp.mod", "example.com/lib", blocks["example.com/lib"])
	if len(rerrs) > 0 {
		t.Fatal(rerrs)
	}
	lib := &CxxLib{Module: "example.com/lib", Version: "v1.0.0", Dir: dir, File: filepath.Join(dir, "gocpp.mod")}
	for _, s := range r.Stmts {
		switch s.Verb {
		case gocppmod.RecipeSrc:
			lib.srcs = append(lib.srcs, s)
		case gocppmod.RecipeExclude:
			lib.excludes = append(lib.excludes, s)
		}
	}
	return lib
}

func writeTree(t *testing.T, dir string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("int x;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func sourceNames(srcs []LibSource) []string {
	var out []string
	for _, s := range srcs {
		out = append(out, s.Rel)
	}
	return out
}

func errMsgs(lib *CxxLib) []string {
	_, errs := lib.Sources()
	var out []string
	for _, e := range errs {
		out = append(out, fmt.Sprintf("%d: %s", e.Pos.Line, e.Msg))
	}
	return out
}

func TestLibSourcesPatterns(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.c", "b.cpp", "c.cc", "d.cxx", "e.C", "f.c++", "g.h", "h.hpp",
		"backends/x.cpp", "backends/y_vulkan.cpp", "backends/z.cpp", "backends/readme.txt", "backends/sub/deep.cpp",
		".hidden/h.c", "src/.dot.c", "src/real.c")
	if err := os.Mkdir(filepath.Join(dir, "adir.c"), 0o755); err != nil { // a folder that a pattern matches: skipped
		t.Fatal(err)
	}
	lib := libOf(t, dir,
		"\tsrc a.c ./b.cpp c.cc",
		"\tsrc backends/*.cpp",
		"\tsrc */real.c d.cxx",
		"\tsrc a.c", // twice: once
		"\texclude backends/*vulkan*",
	)
	srcs, errs := lib.Sources()
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errMsgs(lib))
	}
	want := []string{"a.c", "b.cpp", "backends/x.cpp", "backends/z.cpp", "c.cc", "d.cxx", "src/real.c"}
	if got := sourceNames(srcs); !slices.Equal(got, want) {
		t.Errorf("sources = %q, want %q", got, want)
	}
	for _, s := range srcs {
		if s.C != strings.HasSuffix(s.Rel, ".c") || s.File != filepath.Join(dir, filepath.FromSlash(s.Rel)) || s.Line == 0 || s.Text == "" {
			t.Errorf("source %+v", s)
		}
	}
	if srcs[0].Line != 3 || srcs[0].Text != "src a.c ./b.cpp c.cc" {
		t.Errorf("the first source is from line %d %q", srcs[0].Line, srcs[0].Text)
	}
	// The result is the same for a second call, without looking again.
	again, _ := lib.Sources()
	if len(again) != len(srcs) {
		t.Error("a second call gave other sources")
	}

	// A pattern that starts with a dot matches dot files.
	lib = libOf(t, dir, "\tsrc .hidden/*.c src/.*.c")
	if srcs, errs := lib.Sources(); len(errs) > 0 || !slices.Equal(sourceNames(srcs), []string{".hidden/h.c", "src/.dot.c"}) {
		t.Errorf("dot files: %q %v", sourceNames(srcs), errMsgs(lib))
	}
}

func TestLibSourcesErrors(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, "a.c", "g.h", "sub/x.cpp", "e.C", "f.c++", "empty/readme.txt")
	for _, tc := range []struct{ line, want string }{
		{"src missing.c", `src missing.c: no such file in the module`},
		{"src sub", `src sub: a folder, not a file`},
		{"src g.h", `src g.h: not a C or C++ source (.c, .cc, .cpp, .cxx)`},
		{"src e.C", `not a C or C++ source`},
		{"src f.c++", `not a C or C++ source`},
		{"src nodir/x.c", `no such file`},
		{"src *.cc", `src *.cc: the pattern matches no C or C++ source file`},
		{"src nodir/*.c", `the pattern matches no C or C++ source file`},
		{"src empty/*.c", `the pattern matches no C or C++ source file`},
		{"src *.h", `the pattern matches no C or C++ source file`}, // headers are not sources
	} {
		lib := libOf(t, dir, "\tsrc a.c", "\t"+tc.line)
		got := errMsgs(lib)
		if len(got) != 1 || !strings.Contains(got[0], tc.want) || !strings.HasPrefix(got[0], "4: cxx example.com/lib: ") {
			t.Errorf("%q: errors %q, want one with %q at line 4", tc.line, got, tc.want)
		}
	}
	// src of a module folder that does not exist.
	lib := libOf(t, filepath.Join(dir, "nope"), "\tsrc a.c")
	if got := errMsgs(lib); len(got) != 1 || !strings.Contains(got[0], "the module's folder") {
		t.Errorf("no folder: %q", got)
	}
}

func TestLibSourcesSymlinks(t *testing.T) {
	outside := t.TempDir()
	writeTree(t, outside, "secret.c", "dir/leak.c")
	mod := t.TempDir()
	writeTree(t, mod, "a.c", "real/r.c")
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(mod, name)); err != nil {
			t.Skip("no symbolic links:", err)
		}
	}
	link(filepath.Join(outside, "secret.c"), "leak.c")
	link(outside, "outdir")
	link("real", "alias")        // inside the module: fine
	link("a.c", "inside.c")      // inside the module: fine
	link("../../../../etc", "e") // relative, out of the module
	for _, tc := range []struct{ line, want string }{
		{"src leak.c", "a symbolic link leaves the module"},
		{"src outdir/secret.c", "a symbolic link leaves the module"},
		{"src outdir/*.c", "a symbolic link leaves the module"},
		{"src */secret.c", "a symbolic link leaves the module"},
		{"src *.c", "a symbolic link leaves the module"}, // matches leak.c
	} {
		lib := libOf(t, mod, "\t"+tc.line)
		got := errMsgs(lib)
		if len(got) == 0 || !strings.Contains(got[0], tc.want) {
			t.Errorf("%q: errors %q, want %q", tc.line, got, tc.want)
		}
		if srcs, _ := lib.Sources(); slices.ContainsFunc(srcs, func(s LibSource) bool { return strings.Contains(s.Rel, "secret") || s.Rel == "leak.c" }) {
			// The sources that were found before the error stay, but never the one that leaves.
			t.Errorf("%q: the file outside the module is a source: %v", tc.line, sourceNames(srcs))
		}
	}
	// A link that stays inside the module is fine.
	lib := libOf(t, mod, "\tsrc inside.c alias/r.c")
	srcs, errs := lib.Sources()
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errMsgs(lib))
	}
	// alias/r.c is real/r.c, inside.c is a.c: both are sources, once each.
	if got := sourceNames(srcs); !slices.Equal(got, []string{"alias/r.c", "inside.c"}) {
		t.Errorf("sources = %q", got)
	}
	// The same file by two names is one source.
	lib = libOf(t, mod, "\tsrc a.c inside.c")
	if srcs, _ := lib.Sources(); len(srcs) != 1 {
		t.Errorf("a.c and its link: %v", sourceNames(srcs))
	}
}

func TestLibSourcesBounds(t *testing.T) {
	defer func(a, b, c int) { maxLibSources, maxGlobEntries, maxDirEntries = a, b, c }(maxLibSources, maxGlobEntries, maxDirEntries)
	dir := t.TempDir()
	var files []string
	for i := 0; i < 30; i++ {
		files = append(files, fmt.Sprintf("big/f%02d.c", i))
	}
	for i := 0; i < 5; i++ {
		files = append(files, fmt.Sprintf("d%d/a.c", i), fmt.Sprintf("d%d/b.c", i))
	}
	writeTree(t, dir, files...)

	maxLibSources, maxGlobEntries, maxDirEntries = 20, 1000, 1000
	if got := errMsgs(libOf(t, dir, "\tsrc big/*.c")); len(got) != 1 || !strings.Contains(got[0], "the pattern matches more than 20 files") {
		t.Errorf("too many matches: %q", got)
	}
	maxLibSources, maxGlobEntries, maxDirEntries = 1000, 1000, 20
	if got := errMsgs(libOf(t, dir, "\tsrc big/*.c")); len(got) != 1 || !strings.Contains(got[0], `the folder "big" has more than 20 entries`) {
		t.Errorf("huge folder: %q", got)
	}
	maxLibSources, maxGlobEntries, maxDirEntries = 1000, 40, 1000
	if got := errMsgs(libOf(t, dir, "\tsrc */*.c")); len(got) != 1 || !strings.Contains(got[0], "look at more than 40 directory entries") {
		t.Errorf("many folders: %q", got)
	}
	// Many patterns that each match a few files: the total is bounded too.
	maxLibSources, maxGlobEntries, maxDirEntries = 8, 1000, 1000
	if got := errMsgs(libOf(t, dir, "\tsrc d0/*.c d1/*.c d2/*.c d3/*.c d4/*.c")); len(got) != 1 || !strings.Contains(got[0], "more than 8 source files") {
		t.Errorf("total: %q", got)
	}
	maxLibSources, maxGlobEntries, maxDirEntries = 1000, 1000, 1000
	if got := errMsgs(libOf(t, dir, "\tsrc big/*.c d?/*.c")); len(got) != 0 {
		t.Errorf("within the bounds: %q", got)
	}
}

func TestLibSourcesNoSrc(t *testing.T) {
	lib := libOf(t, t.TempDir())
	if srcs, errs := lib.Sources(); len(srcs) != 0 || len(errs) != 0 {
		t.Errorf("a header-only recipe has no sources: %v %v", srcs, errs)
	}
}

func TestCheckExpandHostile(t *testing.T) {
	// The patterns come from CheckRecipePath, but expand must hold alone.
	dir := t.TempDir()
	writeTree(t, dir, "a.c")
	m, err := newModRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, pat := range []string{"[", "[a-", "a[", "*[", "\\"} {
		if _, err := m.expand(pat, &globBudget{entries: 100}); err == nil && strings.ContainsAny(pat, "*?[") {
			// a bad pattern is an error or matches nothing; both are errors
			t.Errorf("expand(%q) matched something", pat)
		}
	}
}

func TestLibSourcesExtensions(t *testing.T) {
	dir := t.TempDir()
	// Assembly, Objective-C, CUDA and the other spellings of C++ are not
	// sources: gocpp compiles .c, .cc, .cpp and .cxx, as for a package.
	var names []string
	for _, ext := range []string{".S", ".s", ".sx", ".m", ".mm", ".cu", ".C", ".CC", ".cp", ".c++", ".cppm", ".h", ".hpp", ".inl", ".o", ".a", ""} {
		names = append(names, "f"+ext)
	}
	writeTree(t, dir, names...)
	for _, n := range names {
		lib := libOf(t, dir, "\tsrc "+n)
		if got := errMsgs(lib); len(got) != 1 || !strings.Contains(got[0], "not a C or C++ source") {
			t.Errorf("src %s: %q", n, got)
		}
	}
	// ... and a pattern skips them.
	lib := libOf(t, dir, "\tsrc f*")
	if got := errMsgs(lib); len(got) != 1 || !strings.Contains(got[0], "matches no C or C++ source file") {
		t.Errorf("src f*: %q", got)
	}
}

func TestLibSourcesSize(t *testing.T) {
	defer func(n int64) { maxSourceSize = n }(maxSourceSize)
	dir := t.TempDir()
	writeTree(t, dir, "small.c", "big.c")
	if err := os.WriteFile(filepath.Join(dir, "big.c"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	maxSourceSize = 50
	for _, line := range []string{"src big.c", "src b*.c"} {
		got := errMsgs(libOf(t, dir, "\t"+line))
		if len(got) != 1 || !strings.Contains(got[0], "100 bytes: a source of a library is at most 50 bytes") {
			t.Errorf("%s: %q", line, got)
		}
	}
	if got := errMsgs(libOf(t, dir, "\tsrc small.c")); len(got) != 0 {
		t.Errorf("small.c: %q", got)
	}
}
