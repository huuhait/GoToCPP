package load

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests of the loader (job 1.9a) load temporary module trees with a
// fake standard library, and check the packages, their order and the
// errors, which read like the go command's (checked against go 1.25.14).

// fakeStd is a small standard library, as the declaration stubs of step
// 1.9b look: functions without bodies.
var fakeStd = fstest.MapFS{
	"fmt/print.gp": {Data: []byte(`package fmt

func Println(a ...any) (n int, err error)
func Printf(format string, a ...any) (n int, err error)
func Sprint(a ...any) string
func Sprintf(format string, a ...any) string
func Errorf(format string, a ...any) error
`)},
	"errors/errors.gp": {Data: []byte("package errors\n\nfunc New(text string) error\n")},
	"strings/strings.gp": {Data: []byte(`package strings

import "unicode/utf8"

const MaxRune = utf8.RuneSelf

func ToUpper(s string) string
`)},
	"unicode/utf8/utf8.gp": {Data: []byte("package utf8\n\nconst RuneSelf = 0x80\n")},
	// os has per-system files, like the ported standard library.
	"os/file.gp":         {Data: []byte("package os\n\nvar Args []string\n")},
	"os/file_unix.gp":    {Data: []byte("//go:build unix\n\npackage os\n\nconst PathSeparator = '/'\n")},
	"os/file_windows.gp": {Data: []byte("package os\n\nconst PathSeparator = '\\\\'\n")},
	"os/PORT.md":         {Data: []byte("not a .gp file\n")},
	// A folder without .gp files: unicode is "not available yet".
	"unicode/README": {Data: []byte("no stubs yet\n")},
	// The standard library keeps Go's names, also C (unicode.C).
	"cnames/tables.gp": {Data: []byte("package cnames\n\ntype RangeTable struct{ N int }\n\nvar C, L *RangeTable\n")},
	// An internal package, for standard packages only.
	"internal/bytealg/bytealg.gp": {Data: []byte("package bytealg\n\nconst MaxLen = 64\n")},
	"bytes/bytes.gp":              {Data: []byte("package bytes\n\nimport \"internal/bytealg\"\n\nconst MinRead = bytealg.MaxLen\n")},
	// A standard package can't import a module package.
	"badstd/bad.gp": {Data: []byte("package badstd\n\nimport _ \"example.com/app/geom\"\n")},
}

// goStd stands for the list of Go's standard packages (step 1.9b).
func goStd(path string) bool {
	return slices.Contains([]string{"fmt", "errors", "strings", "unicode", "unicode/utf8", "os", "net/http", "unsafe",
		"bytes", "internal/bytealg", "internal/abi"}, path)
}

// writeTree writes the files (slash-separated names relative to the
// folder, and their contents) into a new temporary folder and returns it.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// loadIn loads the patterns in the folder dir with the fake standard
// library, for linux/amd64, with the settings of cfg.
func loadIn(t *testing.T, cfg Config, dir string, patterns ...string) *Program {
	t.Helper()
	cfg.Dir = dir
	if cfg.Stdlib == nil {
		cfg.Stdlib = fakeStd
	}
	if cfg.GoStd == nil {
		cfg.GoStd = goStd
	}
	if cfg.GOOS == "" {
		cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	}
	prog := Load(cfg, patterns...)
	checkProgram(t, prog)
	return prog
}

// checkProgram checks the invariants of a loaded program: every package is
// in Packages once, after the packages it imports (unless they are in a
// cycle); every file has all its fields set, with an absolute name (or one
// in the standard library folder) that is its name in the FileSet; and
// every import is in Imports.
func checkProgram(t *testing.T, prog *Program) {
	t.Helper()
	pos := make(map[*Package]int)
	for i, p := range prog.Packages {
		if _, ok := pos[p]; ok {
			t.Errorf("package %s is twice in Packages", p.ImportPath)
		}
		pos[p] = i
	}
	for _, r := range prog.Roots {
		if _, ok := pos[r]; !ok {
			t.Errorf("root %s is not in Packages", r.ImportPath)
		}
	}
	for _, p := range prog.Packages {
		if p.Errors == nil {
			t.Errorf("package %s: Errors is nil", p.ImportPath)
		}
		for path, dep := range p.Imports {
			if _, ok := pos[dep]; !ok && !dep.summary {
				t.Errorf("package %s imports %s, which is not in Packages", p.ImportPath, path)
			}
			if pos[dep] > pos[p] && !dep.Incomplete {
				t.Errorf("package %s comes before %s, which it imports", p.ImportPath, dep.ImportPath)
			}
		}
		for _, f := range p.Files {
			if f.AST == nil || f.Info == nil || f.Rewrite == nil {
				t.Errorf("file %s: a field is not set: %+v", f.Path, f)
				continue
			}
			if !filepath.IsAbs(f.Path) && !strings.HasPrefix(f.Path, filepath.FromSlash("$GOCPP/stdlib/")) {
				t.Errorf("file name %s is not absolute", f.Path)
			}
			if name := prog.Fset.File(f.AST.FileStart).Name(); name != f.Path {
				t.Errorf("file %s has the name %s in the FileSet", f.Path, name)
			}
			if filepath.Dir(f.Path) != p.Dir {
				t.Errorf("file %s is not in the folder %s of its package", f.Path, p.Dir)
			}
			for _, spec := range f.Info.Packages {
				path, _ := importPath(spec)
				if _, ok := p.Imports[path]; !ok && path != "unsafe" && !p.Errors.HasErrors() {
					t.Errorf("package %s: the import of %s is not in Imports", p.ImportPath, path)
				}
			}
		}
	}
}

// errorText returns the errors of the program as the go command prints
// them: Program.Errors, then for each package with diagnostics a
// "# import path" line and its diagnostics, file names relative to root
// and root itself written as $ROOT.
func errorText(t *testing.T, prog *Program, root string) string {
	t.Helper()
	var b bytes.Buffer
	pr := diag.Printer{All: true, Dir: root}
	if err := pr.Print(&b, prog.Errors); err != nil {
		t.Fatal(err)
	}
	for _, p := range prog.Packages {
		if len(p.Errors.Diagnostics()) == 0 {
			continue
		}
		fmt.Fprintf(&b, "# %s\n", shown(p))
		if err := pr.Print(&b, p.Errors); err != nil {
			t.Fatal(err)
		}
	}
	return slashed(strings.ReplaceAll(b.String(), root, "$ROOT"))
}

// slashed returns s with the folder separators of this system written as
// '/', so that the expected texts of the tests, which write paths with '/',
// also hold on Windows, where the loader (like the go command) prints '\'.
func slashed(s string) string {
	if filepath.Separator == '/' {
		return s
	}
	return strings.ReplaceAll(s, string(filepath.Separator), "/")
}

// noSuchFileText returns the text of this system's error for a file that is
// not there ("no such file or directory" on Linux).
func noSuchFileText(t *testing.T) string {
	t.Helper()
	_, err := os.Open(filepath.Join(t.TempDir(), "missing.gp"))
	if err == nil {
		t.Fatal("a missing file was opened")
	}
	return errors.Unwrap(err).Error()
}

// shown returns the import path of p as go test shows it: with
// "[x.test]" for the packages of the test of x.
func shown(p *Package) string {
	if p.ForTest != "" {
		return p.ImportPath + " [" + p.ForTest + ".test]"
	}
	return p.ImportPath
}

// summary returns one line for each package of the program, in the order
// of Packages: "import path name: files; imports".
func summary(prog *Program) []string {
	var lines []string
	for _, p := range prog.Packages {
		var files []string
		for _, f := range p.Files {
			files = append(files, filepath.Base(f.Path))
		}
		var imports []string
		for path := range p.Imports {
			imports = append(imports, path)
		}
		slices.Sort(imports)
		line := fmt.Sprintf("%s %s: %s", shown(p), p.Name, strings.Join(files, " "))
		if len(imports) > 0 {
			line += "; " + strings.Join(imports, " ")
		}
		lines = append(lines, line)
	}
	return lines
}

// wantSummary fails if the summary of prog is not want.
func wantSummary(t *testing.T, prog *Program, want ...string) {
	t.Helper()
	if got := summary(prog); !slices.Equal(got, want) {
		t.Errorf("packages:\n\t%s\nwant:\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}

// wantErrors fails if the errors of prog are not want.
func wantErrors(t *testing.T, prog *Program, root, want string) {
	t.Helper()
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// root returns the package of prog with the import path path.
func pkgOf(t *testing.T, prog *Program, path string) *Package {
	t.Helper()
	for _, p := range prog.Packages {
		if p.ImportPath == path {
			return p
		}
	}
	t.Fatalf("no package %s", path)
	return nil
}

// app is a small module: main imports geom and the standard library, and
// geom imports fmt.
var app = map[string]string{
	"gocpp.mod": "// The app.\nmodule example.com/app // its path\n\ngocpp 0.1\ncxx c++20\n",
	"main.gp": `package main

import (
	"fmt"
	"strings"

	"example.com/app/geom"
)

func main() { fmt.Println(strings.ToUpper("x"), geom.Rect{W: 1}) }
`,
	"geom/geom.gp": `package geom

import "fmt"

type Rect struct{ W, H int }

func (r Rect) String() string { return fmt.Sprint(r.W, r.H) }
`,
	"geom/area.gp":      "package geom\n\nfunc (r Rect) Area() int { return r.W * r.H }\n",
	"geom/README.md":    "not a .gp file\n",
	"geom/_draft.gp":    "package draft\n",
	"geom/.hidden.gp":   "package hidden\n",
	"geom/sub/x.gp":     "package sub\n",
	"geom/geom_test.gp": "package geom\n\nimport \"testing\"\n",
}

func TestLoadModule(t *testing.T) {
	root := writeTree(t, app)
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"fmt fmt: print.gp",
		"example.com/app/geom geom: area.gp geom.gp; fmt",
		"unicode/utf8 utf8: utf8.gp",
		"strings strings: strings.gp; unicode/utf8",
		"example.com/app main: main.gp; example.com/app/geom fmt strings",
	)
	if len(prog.Roots) != 1 || prog.Roots[0].ImportPath != "example.com/app" {
		t.Fatalf("roots: %v", prog.Roots)
	}
	main := prog.Roots[0]
	if main.Dir != root || main.Module == nil || main.Module.Path != "example.com/app" || main.Module.Dir != root {
		t.Errorf("main: Dir %s, Module %+v", main.Dir, main.Module)
	}
	geom := pkgOf(t, prog, "example.com/app/geom")
	if geom.Dir != filepath.Join(root, "geom") || geom.Module != main.Module || geom.Std {
		t.Errorf("geom: Dir %s, Module %+v, Std %v", geom.Dir, geom.Module, geom.Std)
	}
	if main.Imports["example.com/app/geom"] != geom {
		t.Errorf("main doesn't import the loaded geom")
	}
	fmtPkg := pkgOf(t, prog, "fmt")
	if !fmtPkg.Std || fmtPkg.Module != nil || fmtPkg.Dir != filepath.FromSlash("$GOCPP/stdlib/fmt") {
		t.Errorf("fmt: Std %v, Module %v, Dir %s", fmtPkg.Std, fmtPkg.Module, fmtPkg.Dir)
	}
	if geom.Imports["fmt"] != fmtPkg {
		t.Errorf("geom and main don't share fmt")
	}
	for _, p := range prog.Packages {
		if p.Incomplete {
			t.Errorf("package %s is incomplete", p.ImportPath)
		}
	}
}

// TestLoadOrder checks the order of Packages: Go's initialization order,
// where of the packages whose imports are done the one with the smallest
// import path comes first.
func TestLoadOrder(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module m\n",
		"main.gp":   "package main\n\nimport (\n\t_ \"m/z\"\n\t_ \"m/b\"\n\t_ \"m/a\"\n)\n",
		"z/z.gp":    "package z\n",
		"b/b.gp":    "package b\n\nimport _ \"m/z\"\n",
		"a/a.gp":    "package a\n\nimport _ \"m/b\"\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog, "m/z z: z.gp", "m/b b: b.gp; m/z", "m/a a: a.gp; m/b", "m main: main.gp; m/a m/b m/z")
}

// TestMultiPkgExample loads examples/multi-pkg where it is.
func TestMultiPkgExample(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "examples", "multi-pkg"))
	if err != nil {
		t.Fatal(err)
	}
	prog := loadIn(t, Config{}, dir, ".")
	wantErrors(t, prog, dir, "")
	// Go's order: fmt first (every package imports it), then codes before
	// geom (by import path), then catalog, which needs geom.
	wantSummary(t, prog,
		"fmt fmt: print.gp",
		"example.com/multi-pkg/codes codes: codes.gp; fmt",
		"example.com/multi-pkg/geom geom: geom.gp; fmt",
		"example.com/multi-pkg/catalog catalog: catalog.gp; example.com/multi-pkg/geom fmt",
		"example.com/multi-pkg main: main.gp; example.com/multi-pkg/catalog example.com/multi-pkg/codes example.com/multi-pkg/geom fmt",
	)
	codes := pkgOf(t, prog, "example.com/multi-pkg/codes")
	f := codes.Files[0]
	if inc := f.Info.HeaderNames["luhn"]; inc == nil || inc.Path != "./luhn.h" {
		t.Errorf("codes: the header ./luhn.h is not named luhn: %v", infoLines(f.Info))
	}
	if len(f.Rewrite.Uses) != 2 || f.Rewrite.ImportC == nil {
		t.Errorf("codes: %d rewritten uses, import C %v", len(f.Rewrite.Uses), f.Rewrite.ImportC)
	}
}

// TestRealPackageNames checks that ClassifyImports gets the names from the
// package clauses, where the guess from the path would be wrong: the
// package in myapp/api/v1 is v1, and the one in myapp/util is helpers.
func TestLoadRealPackageNames(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module myapp\n",
		"api/v1/v1.gp": "package v1\n\nfunc Get() int { return 1 }\n",
		"util/util.gp": "package helpers\n\nfunc Help() int { return 2 }\n",
		"main.gp": `package main

import (
	"myapp/api/v1"
	"myapp/util"

	"<v1.h>"
	"<util.h>"
	"<api.h>"
)

func main() { _, _ = v1.Get(), helpers.Help(); util.puts(); api.get() }
`,
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	f := prog.Roots[0].Files[0]
	want := []string{`pkg "myapp/api/v1"`, `pkg "myapp/util"`, `sys <v1.h> none UsedByImport "v1"`, "sys <util.h> util", "sys <api.h> api"}
	if got := infoLines(f.Info); !slices.Equal(got, want) {
		t.Errorf("got:\n\t%s\nwant:\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	if f.Info.PackageNames["v1"] == nil || f.Info.PackageNames["helpers"] == nil {
		t.Errorf("package names: %v", f.Info.PackageNames)
	}
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
}

func TestBuildConstraints(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":         "module m\n",
		"a.gp":              "package p\n",
		"a_linux.gp":        "package p\n",
		"a_windows.gp":      "package p\n",
		"a_amd64.gp":        "package p\n",
		"a_arm64.gp":        "package p\n",
		"a_linux_amd64.gp":  "package p\n",
		"a_linux_arm64.gp":  "package p\n",
		"linux.gp":          "package p\n", // no prefix before _: not a system file
		"tag.gp":            "//go:build foo\n\npackage p\n",
		"notag.gp":          "//go:build !foo\n\npackage p\n",
		"unix.gp":           "//go:build unix\n\npackage p\n",
		"go125.gp":          "//go:build go1.25\n\npackage p\n",
		"go126.gp":          "//go:build go1.26\n\npackage p\n",
		"cgo.gp":            "//go:build cgo\n\npackage p\n",
		"nocgo.gp":          "//go:build !cgo\n\npackage p\n",
		"gocpp.gp":          "//go:build gocpp\n\npackage p\n",
		"gc.gp":             "//go:build gc\n\npackage p\n",
		"ignore.gp":         "//go:build ignore\n\npackage p\n",
		"doc.gp":            "// Package p.\n//go:build ignore\npackage p\n", // in the doc comment: still a constraint
		"plusbuild.gp":      "// +build windows\n\npackage p\n",
		"late.gp":           "package p\n\n//go:build ignore\n", // after the package clause: no constraint
		"x_test.gp":         "package p\n",
		"x_windows_test.gp": "package p\n",
	}
	root := writeTree(t, files)
	for _, tc := range []struct {
		goos, goarch string
		tags         []string
		tests        bool
		want         string
	}{
		{"linux", "amd64", nil, false,
			"a.gp a_amd64.gp a_linux.gp a_linux_amd64.gp go125.gp gocpp.gp late.gp linux.gp nocgo.gp notag.gp unix.gp"},
		{"linux", "amd64", []string{"foo"}, false,
			"a.gp a_amd64.gp a_linux.gp a_linux_amd64.gp go125.gp gocpp.gp late.gp linux.gp nocgo.gp tag.gp unix.gp"},
		{"linux", "arm64", nil, true,
			"a.gp a_arm64.gp a_linux.gp a_linux_arm64.gp go125.gp gocpp.gp late.gp linux.gp nocgo.gp notag.gp unix.gp x_test.gp"},
		{"windows", "amd64", nil, true,
			"a.gp a_amd64.gp a_windows.gp go125.gp gocpp.gp late.gp linux.gp nocgo.gp notag.gp plusbuild.gp x_test.gp x_windows_test.gp"},
		{"darwin", "arm64", nil, false,
			"a.gp a_arm64.gp go125.gp gocpp.gp late.gp linux.gp nocgo.gp notag.gp unix.gp"},
	} {
		t.Run(tc.goos+"_"+tc.goarch+strings.Join(tc.tags, ","), func(t *testing.T) {
			prog := loadIn(t, Config{GOOS: tc.goos, GOARCH: tc.goarch, Tags: tc.tags, Tests: tc.tests}, root, ".")
			wantErrors(t, prog, root, "")
			var got []string
			for _, f := range prog.Roots[0].Files {
				got = append(got, filepath.Base(f.Path))
			}
			if strings.Join(got, " ") != tc.want {
				t.Errorf("files:\n got %s\nwant %s", strings.Join(got, " "), tc.want)
			}
		})
	}
}

// TestBuildConstraintErrors checks broken //go:build lines: the file is
// left out with go/build's message, at the file.
func TestBuildConstraintErrors(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module m\n",
		"ok.gp":     "package p\n",
		"two.gp":    "//go:build linux\n//go:build amd64\n\npackage p\n",
		"bad.gp":    "//go:build linux &&\n\npackage p\n",
		// MatchFile can't read the import: the file is kept, and the
		// parser reports the error.
		"syntax.gp": "package p\n\nimport (\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, `# m
./bad.gp: parsing //go:build line: unexpected end of expression
./two.gp: multiple //go:build comments
./syntax.gp:3:10: expected ')', found 'EOF'
`)
	wantSummary(t, prog, "m p: ok.gp syntax.gp")
	// A package whose only file is broken gets no "no .gp files" error.
	root = writeTree(t, map[string]string{"gocpp.mod": "module m\n", "two.gp": "//go:build linux\n//go:build amd64\n\npackage p\n"})
	prog = loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "# m\n./two.gp: multiple //go:build comments\n")
}

// TestStdlibConstraints checks that the files of a standard package are
// chosen by their build constraints too.
func TestStdlibConstraints(t *testing.T) {
	root := writeTree(t, map[string]string{"main.gp": "package main\n\nimport \"os\"\n\nvar _ = os.PathSeparator\n"})
	for goos, want := range map[string]string{"linux": "file.gp file_unix.gp", "windows": "file.gp file_windows.gp"} {
		prog := loadIn(t, Config{GOOS: goos, GOARCH: "amd64"}, root, ".")
		wantErrors(t, prog, root, "")
		var got []string
		for _, f := range pkgOf(t, prog, "os").Files {
			got = append(got, filepath.Base(f.Path))
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%s: os has %s, want %s", goos, strings.Join(got, " "), want)
		}
		if got := typeCheckProgram(t, prog, root); got != "" {
			t.Errorf("%s: go/types errors:\n%s", goos, got)
		}
	}
}

func TestTestFiles(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":      "module example.com/m\n",
		"geom/geom.gp":   "package geom\n\nfunc Area() int { return 1 }\n",
		"geom/a_test.gp": "package geom\n\nfunc Helper() int { return Area() }\n",
		"geom/x_test.gp": "package geom_test\n\nimport (\n\t\"example.com/m/geom\"\n\t. \"example.com/m/geom\"\n)\n\nvar _ = geom.Area() + Helper()\n",
		"main.gp":        "package main\n\nimport \"example.com/m/geom\"\n\nvar _ = geom.Area()\n",
	}
	root := writeTree(t, files)

	prog := loadIn(t, Config{}, root, "./geom")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog, "example.com/m/geom geom: geom.gp")

	prog = loadIn(t, Config{Tests: true}, root, "./geom")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/m/geom [example.com/m/geom.test] geom: a_test.gp geom.gp",
		"example.com/m/geom_test [example.com/m/geom.test] geom_test: x_test.gp; example.com/m/geom",
	)
	if len(prog.Roots) != 2 || prog.Roots[1].Imports["example.com/m/geom"] != prog.Roots[0] {
		t.Errorf("the external test package doesn't import the package with its test files")
	}
	if !prog.Roots[0].Files[0].Test || prog.Roots[0].Files[1].Test {
		t.Errorf("File.Test is wrong")
	}
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}

	// Tests are only for the packages of the patterns: main's dependency
	// geom has no test files.
	prog = loadIn(t, Config{Tests: true}, root, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog, "example.com/m/geom geom: geom.gp", "example.com/m main: main.gp; example.com/m/geom")

	// A test file with another package name.
	root = writeTree(t, map[string]string{
		"gocpp.mod": "module m\n",
		"a.gp":      "package xt\n",
		"a_test.gp": "package xt_test\n",
		"b_test.gp": "package other\n",
	})
	prog = loadIn(t, Config{Tests: true}, root, ".")
	wantErrors(t, prog, root, "# m [m.test]\nfound packages xt (a.gp) and other (b_test.gp) in $ROOT\n")
}

func TestNoFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":          "module example.com/m\n",
		"empty/README":       "no .gp files\n",
		"onlytest/t_test.gp": "package t\n",
		"tagged/t.gp":        "//go:build ignore\n\npackage t\n",
		"hidden/_t.gp":       "package t\n",
		"main.gp":            "package main\n\nimport (\n\t_ \"example.com/m/empty\"\n\t_ \"example.com/m/onlytest\"\n\t_ \"example.com/m/tagged\"\n)\n",
	})
	for _, tc := range []struct{ pattern, want string }{
		{"./empty", "# example.com/m/empty\nno .gp files in $ROOT/empty\n"},
		{"./hidden", "# example.com/m/hidden\nno .gp files in $ROOT/hidden\n"},
		{"./onlytest", "# example.com/m/onlytest\nexample.com/m/onlytest: no non-test .gp files in $ROOT/onlytest\n"},
		{"./tagged", "# example.com/m/tagged\npackage example.com/m/tagged: build constraints exclude all .gp files in $ROOT/tagged\n"},
		// For an imported package, the error is at the import, like the go
		// command does for "found packages".
		{".", "# example.com/m/empty\n./main.gp:4:2: no .gp files in $ROOT/empty\n" +
			"# example.com/m/onlytest\n./main.gp:5:2: example.com/m/onlytest: no non-test .gp files in $ROOT/onlytest\n" +
			"# example.com/m/tagged\n./main.gp:6:2: package example.com/m/tagged: build constraints exclude all .gp files in $ROOT/tagged\n"},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			prog := loadIn(t, Config{}, root, tc.pattern)
			wantErrors(t, prog, root, tc.want)
			want := map[string]NoFilesReason{
				"example.com/m/empty":    NoGpFiles,
				"example.com/m/hidden":   NoGpFiles,
				"example.com/m/onlytest": OnlyTestFiles,
				"example.com/m/tagged":   ExcludedFiles,
			}
			for _, p := range prog.Packages {
				if p.NoFiles != want[p.ImportPath] {
					t.Errorf("%s: NoFiles is %v, want %v", p.ImportPath, p.NoFiles, want[p.ImportPath])
				}
			}
		})
	}
	// With Tests, a folder of test files is a package.
	prog := loadIn(t, Config{Tests: true}, root, "./onlytest")
	wantErrors(t, prog, root, "")
	if prog.Roots[0].NoFiles != HasFiles {
		t.Errorf("onlytest with Tests: NoFiles is %v", prog.Roots[0].NoFiles)
	}

	// A folder whose only file has a broken //go:build line, or can't be
	// read, has files, with an error: NoFiles is not set.
	root = writeTree(t, map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"broken/b.gp": "//go:build linux &&\n\npackage b\n",
		"unread/u.gp": "package u\n",
	})
	unreadable := filepath.Join(root, "unread", "u.gp")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	_, err := os.ReadFile(unreadable)
	for _, pattern := range []string{"./broken", "./unread"} {
		if pattern == "./unread" && err == nil {
			continue // the test runs as root, or on windows: the file can be read
		}
		prog := loadIn(t, Config{}, root, pattern)
		if p := prog.Roots[0]; p.NoFiles != HasFiles || !p.Errors.HasErrors() {
			t.Errorf("%s: NoFiles %v, errors %v; want no NoFiles and an error", pattern, p.NoFiles, p.Errors.Diagnostics())
		}
	}
}

func TestFindModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":            "// The app.\nmodule example.com/app // its path\n",
		"sub/x.gp":             "package sub\n",
		"inner/gocpp.mod":      "module example.com/inner\n",
		"inner/deep/x.gp":      "package deep\n",
		"broken/gocpp.mod":     "go 1.25\n",
		"broken/deeper/README": "x\n",
	})
	for _, tc := range []struct{ dir, root, path string }{
		{".", ".", "example.com/app"},
		{"sub", ".", "example.com/app"},
		{"nosuch", ".", "example.com/app"},
		{"inner/deep", "inner", "example.com/inner"},
		{"broken/deeper", "broken", ""},
	} {
		m := FindModule(filepath.Join(root, filepath.FromSlash(tc.dir)))
		if want := filepath.Join(root, filepath.FromSlash(tc.root)); m == nil || m.Dir != want || m.Path != tc.path {
			t.Errorf("FindModule(%s) = %+v, want {Path:%s Dir:%s}", tc.dir, m, tc.path, want)
		}
	}
	if m := FindModule(t.TempDir()); m != nil {
		t.Errorf("FindModule of a folder outside every module = %+v, want nil", m)
	}
}

func TestFoundPackages(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module m\n",
		"lib/x.gp":  "package lib\n",
		"lib/y.gp":  "package lib2\n",
		"lib/z.gp":  "package lib3\n",
		"main.gp":   "package main\n\nimport \"m/lib\"\n\nvar _ = lib.X\n",
	})
	prog := loadIn(t, Config{}, root, "./lib")
	wantErrors(t, prog, root, "# m/lib\nfound packages lib (x.gp) and lib2 (y.gp) in $ROOT/lib\n")
	if p := prog.Roots[0]; p.Name != "lib" || len(p.Files) != 3 {
		t.Errorf("name %s, %d files", p.Name, len(p.Files))
	}
	prog = loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "# m/lib\n./main.gp:3:8: found packages lib (x.gp) and lib2 (y.gp) in $ROOT/lib\n")
	if main := pkgOf(t, prog, "m"); !main.Incomplete || main.Errors.HasErrors() {
		t.Errorf("main: Incomplete %v, errors %v", main.Incomplete, main.Errors.Diagnostics())
	}
}

func TestCommandLineFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":     "module m\n",
		"a.gp":          "package main\n\nimport \"m/lib\"\n\nfunc main() { lib.F() }\n",
		"gen.gp":        "//go:build ignore\n\npackage main\n",
		"b_test.gp":     "package main\n",
		"other.gp":      "package other\n",
		"lib/lib.gp":    "package lib\n\nfunc F() {}\n",
		"sub/s.gp":      "package main\n",
		"dir.gp/x.gp":   "package x\n",
		"notes.txt":     "",
		"lib/b_test.gp": "package lib\n",
	})
	// Build constraints don't apply to named files, like in go run.
	prog := loadIn(t, Config{}, root, "a.gp", "gen.gp", "b_test.gp")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog, "m/lib lib: lib.gp", "command-line-arguments main: a.gp gen.gp; m/lib")
	if p := prog.Roots[0]; p.Dir != root || p.Module == nil || p.Module.Path != "m" {
		t.Errorf("Dir %s, Module %+v", p.Dir, p.Module)
	}
	prog = loadIn(t, Config{Tests: true}, root, "gen.gp", "b_test.gp", filepath.Join(root, "a.gp"))
	wantErrors(t, prog, root, "")
	wantSummary(t, prog, "m/lib lib: lib.gp", "command-line-arguments [command-line-arguments.test] main: gen.gp b_test.gp a.gp; m/lib")

	for _, tc := range []struct {
		files []string
		want  string
	}{
		{[]string{"a.gp", "sub/s.gp"}, "named files must all be in one directory; have . and sub\n"},
		{[]string{"a.gp", "notes.txt"}, "named files must be .gp files: notes.txt\n"},
		{[]string{"a.gp", "./lib"}, "named files must be .gp files: ./lib\n"},
		{[]string{"dir.gp"}, "dir.gp is a directory, should be a .gp file\n"},
		{[]string{"a.gp", "other.gp"}, "# command-line-arguments\nfound packages main (a.gp) and other (other.gp) in $ROOT\n"},
		{[]string{"b_test.gp"}, "# command-line-arguments\ncommand-line-arguments: no non-test .gp files in $ROOT\n"},
		{[]string{"missing.gp"}, "# command-line-arguments\nopen $ROOT/missing.gp: " + noSuchFileText(t) + "\n"},
	} {
		t.Run(strings.Join(tc.files, " "), func(t *testing.T) {
			prog := loadIn(t, Config{}, root, tc.files...)
			wantErrors(t, prog, root, tc.want)
		})
	}
}

func TestMissingPackages(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"geom/geom.gp": "package geom\n",
		"main.gp": `package main

import (
	_ "net/http"
	_ "fmtx"
	_ "example.com/app/nope"
	_ "example.com/other"
	_ "unicode"
	_ "."
	_ ".."
	_ "/abs"
	_ "x//y"
	_ "fmt/"
	_ "x/../y"
	_ "badstd"
	_ "./x"
	_ "C"
	C "fmtx"
	"unsafe"
)

var _ = unsafe.Sizeof(0)
`,
		"two.gp": "package main\n\nimport _ \"fmtx\"\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	if !prog.HasErrors() {
		t.Errorf("HasErrors is false")
	}
	wantErrors(t, prog, root, `# badstd
$GOCPP/stdlib/badstd/bad.gp:3:8: package example.com/app/geom is not in std ($GOCPP/stdlib/example.com/app/geom)
# example.com/app
./main.gp:4:2: package net/http is not available yet (Phase 09)
./main.gp:5:2: package fmtx is not in std ($GOCPP/stdlib/fmtx)
./main.gp:6:2: package example.com/app/nope is not in module example.com/app ($ROOT/nope)
./main.gp:7:2: no required module provides package example.com/other
./main.gp:8:2: package unicode is not available yet (Phase 09)
./main.gp:9:2: "." is relative, but relative import paths are not supported
./main.gp:10:2: ".." is relative, but relative import paths are not supported
./main.gp:11:2: "/abs" is not a package path
./main.gp:12:2: malformed import path "x//y": double slash
./main.gp:13:2: malformed import path "fmt/": trailing slash
./main.gp:14:2: malformed import path "x/../y": invalid path element ".."
./main.gp:16:4: invalid import path "./x": a local header ends in .h, .hh, .hpp, .hxx, .h++ or .inl
./main.gp:17:4: import "C" is not allowed: C's built-in types need no import in gocpp, and there is no inline C preamble (put C code in a header next to the .gp file)
./main.gp:18:2: cannot import package as C - C is reserved for C's built-in types
`)
	main := prog.Roots[0]
	if len(main.Imports) != 1 || main.Imports["badstd"] == nil || !main.Incomplete {
		t.Errorf("main: imports %v, Incomplete %v", main.Imports, main.Incomplete)
	}
	// The error of a path is reported once, at its first import; in two.gp
	// fmtx is not reported again.
	if strings.Contains(errorText(t, prog, root), "two.gp") {
		t.Errorf("fmtx is reported twice")
	}

	// An import that ClassifyImports rejects is not in Imports.
	root = writeTree(t, map[string]string{"main.gp": "package main\n\nimport C \"fmt\"\n"})
	prog = loadIn(t, Config{}, root, ".")
	if len(prog.Roots[0].Imports) != 0 {
		t.Errorf("imports: %v", prog.Roots[0].Imports)
	}

	// A C "fmtx" import before the plain one: the error is at the import
	// that is a gocpp package.
	root = writeTree(t, map[string]string{"main.gp": "package main\n\nimport (\n\tC \"fmtx\"\n\t_ \"fmtx\"\n)\n"})
	prog = loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, `# command-line-arguments
./main.gp:4:2: cannot import package as C - C is reserved for C's built-in types
./main.gp:5:2: package fmtx is not in std ($GOCPP/stdlib/fmtx)
`)
}

// TestNoModule checks a program without gocpp.mod: it can import only
// standard packages.
func TestNoModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"main.gp": "package main\n\nimport (\n\t\"fmt\"\n\t_ \"example.com/q\"\n\t_ \"myapp/x\"\n)\n\nfunc main() { fmt.Println() }\n",
		"x/x.gp":  "package x\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, `# command-line-arguments
./main.gp:5:2: no required module provides package example.com/q: gocpp.mod file not found in $ROOT or any parent directory
./main.gp:6:2: package myapp/x is not in std ($GOCPP/stdlib/myapp/x)
`)
	if p := prog.Roots[0]; p.ImportPath != "command-line-arguments" || p.Module != nil {
		t.Errorf("import path %s, module %v", p.ImportPath, p.Module)
	}
	// Without a standard library and a list of Go's packages.
	prog = Load(Config{Dir: root}, ".")
	wantErrors(t, prog, root, `# command-line-arguments
./main.gp:4:2: package fmt is not in std ($GOCPP/stdlib/fmt)
./main.gp:5:2: no required module provides package example.com/q: gocpp.mod file not found in $ROOT or any parent directory
./main.gp:6:2: package myapp/x is not in std ($GOCPP/stdlib/myapp/x)
`)
}

func TestImportCycles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   "package main\n\nimport _ \"example.com/m/a\"\n",
		"a/a.gp":    "package a\n\nimport _ \"example.com/m/b\"\n",
		"b/b.gp":    "package b\n\nimport _ \"example.com/m/c\"\n",
		"c/c.gp":    "package c\n\nimport _ \"example.com/m/a\"\n",
		"x/x.gp":    "package x\n\nimport _ \"example.com/m/y\"\n",
		"y/y.gp":    "package y\n\nimport \"example.com/m/x\"\n\nvar _ = x.X\n",
		"s/s.gp":    "package s\n\nimport _ \"example.com/m/s\"\n",
	})
	for _, tc := range []struct{ pattern, want string }{
		// The messages of go build 1.25.14 for the same packages.
		{".", `# example.com/m/a
package example.com/m
	imports example.com/m/a from main.gp
	imports example.com/m/b from a.gp
	imports example.com/m/c from b.gp
	imports example.com/m/a from c.gp: import cycle not allowed
`},
		{"./a", `# example.com/m/a
package example.com/m/a
	imports example.com/m/b from a.gp
	imports example.com/m/c from b.gp
	imports example.com/m/a from c.gp: import cycle not allowed
`},
		{"./x", `# example.com/m/x
package example.com/m/x
	imports example.com/m/y from x.gp
	imports example.com/m/x from y.gp: import cycle not allowed
`},
		{"./s", `# example.com/m/s
package example.com/m/s
	imports example.com/m/s from s.gp: import cycle not allowed
`},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			prog := loadIn(t, Config{}, root, tc.pattern)
			wantErrors(t, prog, root, tc.want)
			for _, p := range prog.Packages {
				if !p.Incomplete {
					t.Errorf("package %s is not incomplete", p.ImportPath)
				}
			}
		})
	}
	// The packages of the cycle come at the end, in the order they
	// started loading; the name of x (from its package clause) reached y.
	prog := loadIn(t, Config{}, root, "./x")
	wantSummary(t, prog, "example.com/m/x x: x.gp; example.com/m/y", "example.com/m/y y: y.gp; example.com/m/x")
	y := pkgOf(t, prog, "example.com/m/y")
	if y.Files[0].Info.PackageNames["x"] == nil {
		t.Errorf("y: package names %v", y.Files[0].Info.PackageNames)
	}
}

// TestLoadDotImports checks that the exported names of dot-imported
// packages reach ClassifyImports and RewriteFile: they are no namespaces,
// and a header doesn't get one as its name.
func TestLoadDotImports(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"geom/geom.gp": "package geom\n\ntype Point struct{ X, Y int }\n\nfunc (p Point) Sum() int { return p.X + p.Y }\n\nvar Origin Point\n\nfunc helper() {}\n",
		"main.gp": `package main

import (
	. "example.com/app/geom"

	"<vector>"
	"<Point.h>"
)

var sum = Point.Sum       // a method expression, not a namespace
var o = Origin.X          // not a namespace either
var v std.vector[int]     // a namespace
var h = helper.x          // helper is not exported: a namespace

func main() { _, _, _, _ = sum, o, v, h }
`,
		"named.gp": `package main

import (
	. "example.com/app/geom"

	Origin "<origin.h>"
)
`,
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, `# example.com/app
./named.gp:6:2: Origin redeclared in this block
	./named.gp:4:2: other declaration of Origin
`)
	main := prog.Roots[0]
	f := main.Files[0]
	if inc := f.Info.Includes[1]; inc.Name != "" || inc.Why != UsedByImport {
		t.Errorf("<Point.h>: name %q, %v", inc.Name, inc.Why)
	}
	var ns []string
	for _, c := range f.Rewrite.Namespaces {
		ns = append(ns, c.Name)
	}
	if strings.Join(ns, " ") != "std helper" {
		t.Errorf("namespaces: %v, want std helper", ns)
	}
}

// TestDotImportUnsafe checks a dot import of unsafe, which has no .gp
// files: its names come from go/types.
func TestDotImportUnsafe(t *testing.T) {
	root := writeTree(t, map[string]string{
		"main.gp": "package main\n\nimport (\n\t. \"unsafe\"\n\n\t\"<Sizeof.h>\"\n)\n\nvar _ = Sizeof(0)\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	if inc := prog.Roots[0].Files[0].Info.Includes[0]; inc.Name != "" || inc.Why != UsedByImport {
		t.Errorf("<Sizeof.h>: name %q, %v", inc.Name, inc.Why)
	}
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
}

func TestGocppMod(t *testing.T) {
	for _, tc := range []struct{ mod, path, want string }{
		{"module example.com/app\n", "example.com/app/sub", ""},
		{"module \"example.com/app\" // a comment\n", "example.com/app/sub", ""},
		{"// no module line\ngocpp 0.1\n", commandLine, "../gocpp.mod: missing module declaration\n"},
		{"module a\nmodule b\n", "a/sub", "../gocpp.mod:2: repeated module statement\n"},
		{"module\n", commandLine, "../gocpp.mod:1: usage: module module/path\n"},
		{"module a b\n", commandLine, "../gocpp.mod:1: usage: module module/path\n"},
		{"module \"a\n", commandLine, "../gocpp.mod:1: invalid quoted string: \"a\n"},
		{"module x//y\n", "x/sub", ""}, // like in go.mod, // starts a comment
		{"module x/../y\n", commandLine, "../gocpp.mod:1: invalid module path \"x/../y\"\n"},
		{"module <x.h>\n", commandLine, "../gocpp.mod:1: invalid module path \"<x.h>\"\n"},
	} {
		t.Run(tc.mod, func(t *testing.T) {
			root := writeTree(t, map[string]string{
				"gocpp.mod":    tc.mod,
				"sub/main.gp":  "package main\n",
				"sub/other.gp": "package main\n",
			})
			sub := filepath.Join(root, "sub")
			prog := loadIn(t, Config{}, sub, ".")
			wantErrors(t, prog, sub, tc.want)
			if prog.HasErrors() != (tc.want != "") {
				t.Errorf("HasErrors is %v", prog.HasErrors())
			}
			if p := prog.Roots[0]; p.ImportPath != tc.path || p.Module == nil || p.Module.Dir != root {
				t.Errorf("import path %s, module %+v", p.ImportPath, p.Module)
			}
		})
	}
}

func TestPatterns(t *testing.T) {
	root := writeTree(t, app)
	prog := loadIn(t, Config{}, root, "fmt", "example.com/app/geom", "./geom", ".", "../"+filepath.Base(root))
	wantErrors(t, prog, root, "")
	var roots []string
	for _, p := range prog.Roots {
		roots = append(roots, p.ImportPath)
	}
	if want := "fmt example.com/app/geom example.com/app"; strings.Join(roots, " ") != want {
		t.Errorf("roots: %s, want %s", strings.Join(roots, " "), want)
	}

	// A package that an earlier root imports is the same package as a root.
	prog = loadIn(t, Config{}, root, ".", "./geom")
	if len(prog.Roots) != 2 || prog.Roots[0].Imports["example.com/app/geom"] != prog.Roots[1] {
		t.Errorf("roots %v: geom is loaded twice", prog.Roots)
	}

	// With Tests, a folder named twice is loaded once.
	prog = loadIn(t, Config{Tests: true}, root, "./geom", "example.com/app/geom", root+"/geom")
	wantErrors(t, prog, root, "# example.com/app/geom [example.com/app/geom.test]\ngeom/geom_test.gp:3:8: package testing is not in std ($GOCPP/stdlib/testing)\n")
	if len(prog.Roots) != 1 || len(prog.Roots[0].Files) != 3 {
		t.Errorf("%d roots, %d files", len(prog.Roots), len(prog.Roots[0].Files))
	}

	for _, tc := range []struct{ pattern, want string }{
		{"./...", "pattern ./...: patterns with ... are not supported yet\n"},
		{"./nothere", "stat $ROOT/nothere: directory not found\n"},
		{"./main.gp/", "$ROOT/main.gp is not a directory\n"},
		{"fmtx", "package fmtx is not in std ($GOCPP/stdlib/fmtx)\n"},
		{"example.com/app/nope", "package example.com/app/nope is not in module example.com/app ($ROOT/nope)\n"},
		{"unsafe", "package unsafe has no .gp files: go/types knows it\n"},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			prog := loadIn(t, Config{}, root, tc.pattern)
			wantErrors(t, prog, root, tc.want)
		})
	}
}

// TestMainNotImportable checks the import of a package main.
func TestMainNotImportable(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":   "module myapp\n",
		"mp/m.gp":     "package main\n\nfunc main() {}\n",
		"q/q.gp":      "package q\n\nimport \"myapp/mp\"\n\nvar _ = mp.X\n",
		"t/t.gp":      "package main\n\nfunc F() {}\n\nfunc main() {}\n",
		"t/x_test.gp": "package main_test\n\nimport \"myapp/t\"\n\nvar _ = main.F\n",
	})
	prog := loadIn(t, Config{}, root, "./q")
	wantErrors(t, prog, root, "# myapp/q\nq/q.gp:3:8: import \"myapp/mp\" is a program, not an importable package\n")
	// An external test of package main imports it.
	prog = loadIn(t, Config{Tests: true}, root, "./t")
	wantErrors(t, prog, root, "")
}

// TestParseErrors checks that syntax errors are in the package's errors and
// that the other steps still run on the partial AST.
func TestParseErrors(t *testing.T) {
	root := writeTree(t, map[string]string{
		"main.gp":  "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println( }\n",
		"other.gp": "//gocpp:lnik m\npackage main\n\nfunc C() {}\n",
		"nopkg.gp": "not go\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, `# command-line-arguments
./main.gp:5:28: expected operand, found '}'
./nopkg.gp:1:1: expected 'package', found not
./other.gp:1:1: unknown //gocpp: directive "lnik"; did you mean //gocpp:link?
./other.gp:4:6: cannot declare C - C is reserved for C's built-in types
`)
	if p := prog.Roots[0]; p.Name != "main" || len(p.Files) != 3 || p.Imports["fmt"] == nil {
		t.Errorf("name %q, %d files, imports %v", p.Name, len(p.Files), p.Imports)
	}
}

// TestAllErrors checks Config.AllErrors: all syntax errors. Without it,
// the parser reports one error per line and stops after the 11th.
func TestAllErrors(t *testing.T) {
	var src strings.Builder
	src.WriteString("package main\n\n")
	for range 12 {
		src.WriteString("var = 1\n")
	}
	root := writeTree(t, map[string]string{"main.gp": src.String()})
	count := func(prog *Program) int { return len(prog.Roots[0].Errors.Diagnostics()) }
	if n := count(loadIn(t, Config{}, root, ".")); n != 11 {
		t.Errorf("%d errors, want 11", n)
	}
	if n := count(loadIn(t, Config{AllErrors: true}, root, ".")); n != 24 {
		t.Errorf("AllErrors: %d errors, want 24 (two on each line)", n)
	}
}

// TestDirectivesJoined checks that each file has its directives.
func TestDirectivesJoined(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a.gp": "//gocpp:link m\n//gocpp:extern-c <lua.h>\npackage main\n\nimport \"<lua.h>\"\n",
		"b.gp": "//gocpp:pkg-config sdl2\npackage main\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	var got []string
	for _, f := range prog.Roots[0].Files {
		for _, d := range f.Directives {
			got = append(got, filepath.Base(f.Path)+" "+d.Name+" "+strings.Join(d.Args, ","))
		}
	}
	if want := "a.gp link m|a.gp extern-c <lua.h>|b.gp pkg-config sdl2"; strings.Join(got, "|") != want {
		t.Errorf("directives %q, want %q", strings.Join(got, "|"), want)
	}
}

// TestFset checks Config.Fset and the names of standard library files.
func TestFset(t *testing.T) {
	root := writeTree(t, map[string]string{"main.gp": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n"})
	fset := token.NewFileSet()
	// An absolute folder on every system (a leading '/' is not absolute on Windows).
	stdlibDir := filepath.Join(root, "opt", "gocpp", "stdlib")
	prog := loadIn(t, Config{Fset: fset, StdlibDir: stdlibDir}, root, ".")
	if prog.Fset != fset {
		t.Errorf("Load made a new FileSet")
	}
	var names []string
	fset.Iterate(func(f *token.File) bool {
		names = append(names, f.Name())
		return true
	})
	want := []string{filepath.Join(root, "main.gp"), filepath.Join(stdlibDir, "fmt", "print.gp")}
	if !slices.Equal(names, want) {
		t.Errorf("files %q, want %q", names, want)
	}
}

// typeCheckProgram type-checks the packages of prog in their order with stock
// go/types and FakeImportC, as step 1.10 will do, and returns the errors as
// diag.Printer prints them, with file names relative to root and root
// itself written as $ROOT (like errorText). The importer gives each package
// the packages that it imports, checked before it.
func typeCheckProgram(t *testing.T, prog *Program, root string) string {
	t.Helper()
	checked := make(map[*Package]*types.Package)
	var b bytes.Buffer
	for _, p := range prog.Packages {
		var errs diag.List
		conf := types.Config{
			GoVersion:   "go1.25",
			FakeImportC: true,
			Importer: importerFunc(func(path string) (*types.Package, error) {
				if path == "unsafe" {
					return types.Unsafe, nil
				}
				if tp := checked[p.Imports[path]]; tp != nil {
					return tp, nil
				}
				return nil, fmt.Errorf("package %s is not checked", path)
			}),
			Error: errs.Collect,
		}
		files := make([]*ast.File, len(p.Files))
		for i, f := range p.Files {
			files[i] = f.AST
		}
		tp, _ := conf.Check(p.ImportPath, prog.Fset, files, nil)
		checked[p] = tp
		if err := (&diag.Printer{All: true, Dir: root}).Print(&b, &errs); err != nil {
			t.Fatal(err)
		}
	}
	return slashed(strings.ReplaceAll(b.String(), root, "$ROOT"))
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// TestTypeCheck is the end of the parse-level pipeline: after Load, stock
// go/types type-checks the example programs and a file with headers and
// namespaces with no errors.
func TestTypeCheck(t *testing.T) {
	examples, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	headers := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/shapes\n",
		"main.gp": `//gocpp:link m
package main

import (
	"fmt"

	"example.com/shapes/area"

	"<cstdio>"
	"<vector>"
	"./mathx.h"
	fs "::std::filesystem"
)

func main() {
	var v std.vector[int]
	v.push_back(area.Square(3))
	cstdio.printf(C.cstr("%d\n"), C.int(v.size()))
	p := fs.path(C.cstr("/tmp"))
	n := int(mathx.hypot3(1, 2, 2))
	fmt.Println(n, p.string())
}
`,
		"area/area.gp": "package area\n\n// Square returns x*x.\nfunc Square(x int) int { return x * x }\n",
	})
	for _, tc := range []struct{ name, dir string }{
		{"hello", filepath.Join(examples, "hello")},
		{"multi-pkg", filepath.Join(examples, "multi-pkg")},
		{"headers", headers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prog := loadIn(t, Config{}, tc.dir, ".")
			wantErrors(t, prog, tc.dir, "")
			if prog.HasErrors() {
				t.Errorf("HasErrors is true")
			}
			if got := typeCheckProgram(t, prog, tc.dir); got != "" {
				t.Errorf("go/types errors:\n%s", got)
			}
		})
	}

	// The check finds real mistakes: a typo in a namespace-free file and a
	// wrong argument to a function of another package.
	bad := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/bad\n",
		"main.gp":      "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/bad/area\"\n)\n\nfunc main() { fmt.Println(count, area.Square(\"x\")) }\n",
		"area/area.gp": "package area\n\nfunc Square(x int) int { return x * x }\n",
	})
	prog := loadIn(t, Config{}, bad, ".")
	wantErrors(t, prog, bad, "")
	want := "./main.gp:9:27: undefined: count\n" +
		"./main.gp:9:46: cannot use \"x\" (untyped string constant) as int value in argument to area.Square\n"
	if got := typeCheckProgram(t, prog, bad); got != want {
		t.Errorf("go/types errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestTestVariants checks the import graph of a test (Config.Tests): the
// test variant of p takes the place of p for every package of the test,
// also for q, which the external test imports and which imports p; a
// package that doesn't import p (r) is shared. go vet accepts the same
// packages.
func TestTestVariants(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":      "module example.com/m6\n",
		"p/p.gp":         "package p\n\nimport \"example.com/m6/r\"\n\ntype T int\n\nvar N = r.R\n",
		"p/in_test.gp":   "package p\n\nimport \"example.com/m6/r\"\n\nvar Extra T = T(r.R)\n",
		"p/x_test.gp":    "package p_test\n\nimport (\n\t\"example.com/m6/p\"\n\t\"example.com/m6/q\"\n)\n\nvar _ p.T = q.V + p.Extra\n",
		"q/q.gp":         "package q\n\nimport (\n\t\"example.com/m6/p\"\n\t\"example.com/m6/r\"\n)\n\nvar V p.T = p.T(r.R)\n",
		"r/r.gp":         "package r\n\nconst R = 1\n",
		"s/s.gp":         "package s\n",
		"s/s_test.gp":    "package s\n\nimport \"example.com/m6/r\"\n\nvar _ = r.R\n",
		"main/main.gp":   "package main\n\nimport _ \"example.com/m6/q\"\n\nfunc main() {}\n",
		"main/m_test.gp": "package main\n",
	}
	root := writeTree(t, files)
	prog := loadIn(t, Config{Tests: true}, root, "./p", "./s", "./main")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/m6/r r: r.gp",
		"example.com/m6/p [example.com/m6/p.test] p: in_test.gp p.gp; example.com/m6/r",
		"example.com/m6/p p: p.gp; example.com/m6/r",
		"example.com/m6/q [example.com/m6/p.test] q: q.gp; example.com/m6/p example.com/m6/r",
		"example.com/m6/p_test [example.com/m6/p.test] p_test: x_test.gp; example.com/m6/p example.com/m6/q",
		"example.com/m6/q q: q.gp; example.com/m6/p example.com/m6/r",
		"example.com/m6/main [example.com/m6/main.test] main: m_test.gp main.gp; example.com/m6/q",
		"example.com/m6/s [example.com/m6/s.test] s: s.gp s_test.gp; example.com/m6/r",
	)
	// The test of p has one p, its test variant, so go/types sees one type
	// p.T; main's test (it doesn't test p) uses the plain p and q.
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
	ptest, xt := prog.Roots[0], prog.Roots[1]
	q := xt.Imports["example.com/m6/q"]
	if xt.Imports["example.com/m6/p"] != ptest || q.Imports["example.com/m6/p"] != ptest || q.ForTest == "" {
		t.Errorf("the test of p doesn't use its test variant everywhere")
	}
	if r := ptest.Imports["example.com/m6/r"]; r.ForTest != "" || prog.Roots[2].Imports["example.com/m6/r"] != r {
		t.Errorf("r is not shared")
	}
	// main's test shares q with the rest of the program, as it doesn't
	// test p.
	if mq := prog.Roots[3].Imports["example.com/m6/q"]; mq == q || mq.ForTest != "" {
		t.Errorf("main's test uses the q of p's test")
	}
}

// TestTestCycle checks an internal test that imports a package that
// imports the package under test: go vet 1.25.14 gives the same error.
func TestTestCycle(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":   "module example.com/m6\n",
		"p/p.gp":      "package p\n\ntype T int\n",
		"p/p_test.gp": "package p\n\nimport \"example.com/m6/q\"\n\nvar _ = q.V\n",
		"q/q.gp":      "package q\n\nimport \"example.com/m6/p\"\n\nvar V p.T\n",
	})
	prog := loadIn(t, Config{Tests: true}, root, "./p")
	wantErrors(t, prog, root, `# example.com/m6/p [example.com/m6/p.test]
package example.com/m6/p
	imports example.com/m6/q from p_test.gp
	imports example.com/m6/p from q.gp: import cycle not allowed in test
`)
	for _, p := range prog.Packages {
		if !p.Incomplete {
			t.Errorf("package %s is not incomplete", shown(p))
		}
	}
	// Without the test, there is no cycle.
	prog = loadIn(t, Config{}, root, "./p", "./q")
	wantErrors(t, prog, root, "")
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
}

// TestOneCycleError checks that a package where two imports close a cycle
// gets one error, the first, like in go build.
func TestOneCycleError(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module m\n",
		"a/a.gp":    "package a\n\nimport (\n\t_ \"m/b\"\n\t_ \"m/c\"\n)\n",
		"b/b.gp":    "package b\n\nimport _ \"m/a\"\n",
		"c/c.gp":    "package c\n\nimport _ \"m/a\"\n",
	})
	prog := loadIn(t, Config{}, root, "./a")
	wantErrors(t, prog, root, `# m/a
package m/a
	imports m/b from a.gp
	imports m/a from b.gp: import cycle not allowed
`)
}

// TestStdlibNamedC checks that the standard library may declare C (Go's
// unicode.C), which the packages that import it can use as a selector,
// but not dot-import.
func TestStdlibNamedC(t *testing.T) {
	root := writeTree(t, map[string]string{
		"main.gp": "package main\n\nimport \"cnames\"\n\nvar _ = cnames.C.N + C.int(1)\n\nfunc main() {}\n",
		"dot.gp":  "package main\n\nimport (\n\t. \"cnames\"\n\n\t\"<L.h>\"\n)\n\nvar _ = L\n",
	})
	prog := loadIn(t, Config{}, root, "./main.gp")
	wantErrors(t, prog, root, "")
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
	if p := pkgOf(t, prog, "cnames"); p.Incomplete {
		t.Errorf("cnames is incomplete: %v", p.Errors.Diagnostics())
	}

	prog = loadIn(t, Config{}, root, "./dot.gp")
	wantErrors(t, prog, root, "# command-line-arguments\n"+
		"./dot.gp:4:2: cannot dot-import \"cnames\": it exports C, which is reserved for C's built-in types\n")
	f := prog.Roots[0].Files[0]
	if inc := f.Info.Includes[0]; inc.Why != UsedByImport {
		t.Errorf("<L.h> gets the name of a dot-imported name: %q %v", inc.Name, inc.Why)
	}
}

// TestStdlibExternalTestMayDotImportC checks that the external test of a standard package may dot-import the
// package although it exports C (Go's tests of unicode do), the way tests/stdgo runs them (GOCPP_TEST_STDLIB).
func TestStdlibExternalTestMayDotImportC(t *testing.T) {
	std := maps.Clone(fakeStd)
	std["cnames/x_test.gp"] = &fstest.MapFile{Data: []byte("package cnames_test\n\nimport . \"cnames\"\n\nvar _ = L\n")}
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n"})

	prog := loadIn(t, Config{Tests: true, Stdlib: std, GoStd: goStd}, root, "cnames")

	wantErrors(t, prog, root, "")
}

// TestUserExternalTestMayNotDotImportC checks that the exemption of the standard library's tests is not one
// for a module's: its external test package still gets the error.
func TestUserExternalTestMayNotDotImportC(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":      "module example.com/m\n",
		"geom/geom.gp":   "package geom\n",
		"geom/x_test.gp": "package geom_test\n\nimport . \"cnames\"\n\nvar _ = L\n",
	})

	prog := loadIn(t, Config{Tests: true}, root, "./geom")

	if got := errorText(t, prog, root); !strings.Contains(got, "cannot dot-import \"cnames\": it exports C") {
		t.Errorf("errors:\n%s\nwant the dot-import of C to be refused", got)
	}
}

// TestNestedModule checks a folder of the module that has a gocpp.mod file
// of its own: it is another module.
func TestNestedModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":   "module example.com/m9\n",
		"main.gp":     "package main\n\nimport _ \"example.com/m9/n/sub\"\nimport _ \"example.com/m9/n\"\n",
		"n/gocpp.mod": "module example.com/m9/n\n",
		"n/n.gp":      "package n\n",
		"n/sub/s.gp":  "package sub\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	// go build's messages: one for an import, one for a pattern.
	wantErrors(t, prog, root, `# example.com/m9
./main.gp:3:8: no required module provides package example.com/m9/n/sub
./main.gp:4:8: no required module provides package example.com/m9/n
`)
	prog = loadIn(t, Config{}, root, "example.com/m9/n/sub")
	wantErrors(t, prog, root, "main module (example.com/m9) does not contain package example.com/m9/n/sub\n")
	prog = loadIn(t, Config{}, root, "./n")
	wantErrors(t, prog, root, "")
	if p := prog.Roots[0]; p.ImportPath != "example.com/m9/n" || p.Module.Dir != filepath.Join(root, "n") {
		t.Errorf("import path %s, module %+v", p.ImportPath, p.Module)
	}
}

// TestInternal checks Go's rule for internal packages, for module and
// standard paths. It comes before "not available yet".
func TestInternal(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":         "module example.com/m\n",
		"a/a.gp":            "package a\n\nimport _ \"example.com/m/a/internal/x\"\n",
		"a/x_test.gp":       "package a_test\n\nimport _ \"example.com/m/a/internal/x\"\n",
		"a/internal/x/x.gp": "package x\n\nimport _ \"example.com/m/a/internal\"\n",
		"a/internal/i.gp":   "package internal\n",
		"a/b/b.gp":          "package b\n\nimport _ \"example.com/m/a/internal/x\"\n",
		"c/c.gp": `package c

import (
	_ "example.com/m/a/internal/x"
	_ "example.com/m/a/internal"
	_ "internal/bytealg"
	_ "internal/abi"
	_ "bytes"
	_ "internal/zzz"
	_ "example.com/m/a/internal/missing"
)
`,
	})
	prog := loadIn(t, Config{}, root, "./a", "./a/b", "./c")
	wantErrors(t, prog, root, `# example.com/m/c
c/c.gp:4:2: use of internal package example.com/m/a/internal/x not allowed
c/c.gp:5:2: use of internal package example.com/m/a/internal not allowed
c/c.gp:6:2: use of internal package internal/bytealg not allowed
c/c.gp:7:2: use of internal package internal/abi not allowed
c/c.gp:9:2: package internal/zzz is not in std ($GOCPP/stdlib/internal/zzz)
c/c.gp:10:2: package example.com/m/a/internal/missing is not in module example.com/m ($ROOT/a/internal/missing)
`)
	// An external test, and .gp files named on the command line, count as
	// their package.
	prog = loadIn(t, Config{Tests: true}, root, "./a")
	wantErrors(t, prog, root, "")
	prog = loadIn(t, Config{}, filepath.Join(root, "a", "b"), "b.gp")
	wantErrors(t, prog, root, "")
}

// TestAllowStdInternal checks Config.AllowStdInternal (gocpp's tests of the standard library's
// internal packages): it opens the internal packages of the standard library that exist, and
// nothing else: a module's internal package stays closed, and so does the hidden package of the
// type checker.
func TestAllowStdInternal(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":         "module example.com/m\n",
		"a/internal/x/x.gp": "package x\n",
		"c/c.gp": `package c

import (
	_ "internal/bytealg"
	_ "internal/abi"
	_ "example.com/m/a/internal/x"
	_ "gocpp/cgoh"
	_ "internal/zzz"
)
`,
	})
	prog := loadIn(t, Config{AllowStdInternal: true}, root, "./c")
	wantErrors(t, prog, root, `# example.com/m/c
c/c.gp:5:2: use of internal package internal/abi not allowed
c/c.gp:6:2: use of internal package example.com/m/a/internal/x not allowed
c/c.gp:7:2: use of internal package gocpp/cgoh not allowed
c/c.gp:8:2: package internal/zzz is not in std ($GOCPP/stdlib/internal/zzz)
`)
	// (internal/abi is a Go package that this test's standard library does not have: it stays
	// closed, as it is for a package that exists in Go and not here.) Off, the standard internal packages are closed again.
	prog = loadIn(t, Config{}, root, "./c")
	wantErrors(t, prog, root, `# example.com/m/c
c/c.gp:4:2: use of internal package internal/bytealg not allowed
c/c.gp:5:2: use of internal package internal/abi not allowed
c/c.gp:6:2: use of internal package example.com/m/a/internal/x not allowed
c/c.gp:7:2: use of internal package gocpp/cgoh not allowed
c/c.gp:8:2: package internal/zzz is not in std ($GOCPP/stdlib/internal/zzz)
`)
}

// TestCgohPath checks that no package can import the hidden package of
// the type checker (CgohPath), also from a module named gocpp that has a
// package with that path; the package itself still loads as a pattern.
func TestCgohPath(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":   "module gocpp\n",
		"main.gp":     "package main\n\nimport _ \"gocpp/cgoh\"\n",
		"cgoh/c.gp":   "package cgoh\n",
		"std/main.gp": "package main\n\nimport \"gocpp/cgoh\"\n",
	})
	prog := loadIn(t, Config{}, root, ".", "./std")
	wantErrors(t, prog, root, `# gocpp
./main.gp:3:8: use of internal package gocpp/cgoh not allowed
# gocpp/std
std/main.gp:3:8: use of internal package gocpp/cgoh not allowed
`)
	prog = loadIn(t, Config{}, root, "./cgoh")
	wantErrors(t, prog, root, "")
}

// TestTestOnlyExternal checks a Tests root without test files of its own
// package: it is the plain package, shared with the rest of the program,
// and only its external test package is for the test, as in go test.
func TestTestOnlyExternal(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":   "module example.com/m6\n",
		"c/c.gp":      "package c\n\ntype T int\n",
		"c/x_test.gp": "package c_test\n\nimport (\n\t\"example.com/m6/c\"\n\t\"example.com/m6/q\"\n)\n\nvar _ c.T = q.V\n",
		"q/q.gp":      "package q\n\nimport \"example.com/m6/c\"\n\nvar V c.T\n",
		"d/d.gp":      "package d\n\nimport _ \"example.com/m6/c\"\n",
		"d/d_test.gp": "package d\n",
	})
	prog := loadIn(t, Config{Tests: true}, root, "./d", "./c")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/m6/c c: c.gp",
		"example.com/m6/d [example.com/m6/d.test] d: d.gp d_test.gp; example.com/m6/c",
		"example.com/m6/q q: q.gp; example.com/m6/c",
		"example.com/m6/c_test [example.com/m6/c.test] c_test: x_test.gp; example.com/m6/c example.com/m6/q",
	)
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
	if c := prog.Roots[1]; c.ForTest != "" || prog.Roots[0].Imports["example.com/m6/c"] != c {
		t.Errorf("c is not the shared plain package")
	}
	// More layouts, each loaded with the plain package loaded by an earlier
	// root (./d ./c), loaded first (./c ./d), and alone (./c).
	for _, tc := range []struct {
		name  string
		files map[string]string // the files of c
		want  string
		xtest bool   // whether there is an external test package
		pkg   string // the name of package c: its first file's (go/build)
	}{{
		name:  "two package clauses, no test files",
		files: map[string]string{"c/b.gp": "package b\n", "c/c.gp": "package c\n"},
		want:  "# example.com/m6/c\n@found packages b (b.gp) and c (c.gp) in $ROOT/c\n",
		pkg:   "b",
	}, {
		name:  "every file excluded, no test files",
		files: map[string]string{"c/c.gp": "//go:build ignore\n\npackage c\n"},
		want:  "# example.com/m6/c\n@package example.com/m6/c: build constraints exclude all .gp files in $ROOT/c\n",
	}, {
		name: "the external test file comes first",
		files: map[string]string{
			"c/a_test.gp": "package foo_test\n",
			"c/c.gp":      "package c\n",
		},
		want:  "# example.com/m6/c_test [example.com/m6/c.test]\nfound packages foo (a_test.gp) and c (c.gp) in $ROOT/c\n",
		xtest: true,
		pkg:   "c",
	}, {
		name: "errors in external test files",
		files: map[string]string{
			"c/c.gp":      "package c\n",
			"c/x_test.gp": "package c_test\n\nimport _ \"example.com/m6/c\"\n\nvar _ = (\n",
			"c/y_test.gp": "//go:build linux &&\n\npackage c_test\n",
			"c/z_test.gp": "package other_test\n",
		},
		want: `# example.com/m6/c_test [example.com/m6/c.test]
found packages c (c.gp) and other (z_test.gp) in $ROOT/c
c/y_test.gp: parsing //go:build line: unexpected end of expression
c/x_test.gp:5:11: expected operand, found 'EOF'
`,
		xtest: true,
		pkg:   "c",
	}, {
		// A test file of the package itself with a broken constraint makes
		// the package a test variant, which keeps the error.
		name: "a broken internal test file",
		files: map[string]string{
			"c/c.gp":      "package c\n",
			"c/w_test.gp": "//go:build linux &&\n\npackage c\n",
		},
		want: "# example.com/m6/c [example.com/m6/c.test]\nc/w_test.gp: parsing //go:build line: unexpected end of expression\n",
		pkg:  "c",
	}, {
		// No files but the external test: d gets "no non-test .gp files"
		// in every order, as with go test.
		name:  "only an external test file",
		files: map[string]string{"c/x_test.gp": "package c_test\n"},
		want:  "# example.com/m6/c\n&example.com/m6/c: no non-test .gp files in $ROOT/c\n",
		xtest: true,
	}, {
		name: "the only other file is excluded",
		files: map[string]string{
			"c/c.gp":      "//go:build ignore\n\npackage c\n",
			"c/x_test.gp": "package c_test\n",
		},
		want:  "# example.com/m6/c\n&example.com/m6/c: no non-test .gp files in $ROOT/c\n",
		xtest: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{
				"gocpp.mod":   "module example.com/m6\n",
				"d/d.gp":      "package d\n\nimport _ \"example.com/m6/c\"\n",
				"d/d_test.gp": "package d\n",
			}
			maps.Copy(files, tc.files)
			root := writeTree(t, files)
			for _, patterns := range [][]string{{"./c"}, {"./d", "./c"}, {"./c", "./d"}} {
				prog := loadIn(t, Config{Tests: true}, root, patterns...)
				// A package-wide error of c is at d's import when d loads
				// c first, and has no position when c is a root first.
				pos := ""
				if patterns[0] == "./d" {
					pos = "d/d.gp:3:8: "
				}
				want := strings.ReplaceAll(tc.want, "@", pos)
				// d imports c in every order, so its error is at d's import;
				// with c alone, nothing imports the plain c.
				if len(patterns) == 1 {
					want = strings.ReplaceAll(want, "# example.com/m6/c\n&example.com/m6/c: no non-test .gp files in $ROOT/c\n", "")
				}
				want = strings.ReplaceAll(want, "&", "d/d.gp:3:8: ")
				wantErrors(t, prog, root, want)
				if got := slices.ContainsFunc(prog.Roots, func(p *Package) bool { return p.Name == "c_test" || p.Name == "foo_test" }); got != tc.xtest {
					t.Errorf("%v: external test package %v, want %v", patterns, got, tc.xtest)
				}
				if c := pkgOf(t, prog, "example.com/m6/c"); c.Name != tc.pkg {
					t.Errorf("%v: c is named %q", patterns, c.Name)
				}
			}
		})
	}
}

// TestRealStdlib loads every package of the real standard library (each
// package of stdlib.Std() that has a stub) for three systems, and
// type-checks them, with 0 errors.
func TestRealStdlib(t *testing.T) {
	var paths []string
	for _, p := range stdlib.Std() {
		entries, _ := fs.ReadDir(stdlib.FS(), p)
		if slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return strings.HasSuffix(e.Name(), ".gp") }) {
			paths = append(paths, p)
		}
	}
	if len(paths) < 20 {
		t.Fatalf("only %d standard packages have stubs: %v", len(paths), paths)
	}
	for _, sys := range [][2]string{{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"}} {
		t.Run(sys[0], func(t *testing.T) {
			dir := t.TempDir()
			cfg := Config{Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: sys[0], GOARCH: sys[1]}
			paths := paths
			if sys[0] != "windows" {
				// The packages of stdlib.WindowsOnly have files for Windows only.
				paths = slices.DeleteFunc(slices.Clone(paths), func(p string) bool { return slices.Contains(stdlib.WindowsOnly, p) })
			}
			if sys[0] == "darwin" {
				// os and what it stands on have files for Linux and Windows only (stdlib.LinuxOnly: macOS is
				// not a target), and so have the packages that import them.
				probe := loadIn(t, cfg, t.TempDir(), paths...)
				var uses func(p *Package, seen map[*Package]bool) bool
				uses = func(p *Package, seen map[*Package]bool) bool {
					if seen[p] {
						return false
					}
					seen[p] = true
					if slices.Contains(stdlib.LinuxOnly, p.ImportPath) {
						return true
					}
					for _, q := range p.Imports {
						if uses(q, seen) {
							return true
						}
					}
					return false
				}
				paths = slices.DeleteFunc(slices.Clone(paths), func(path string) bool {
					for _, r := range probe.Roots {
						if r.ImportPath == path {
							return uses(r, map[*Package]bool{})
						}
					}
					return false
				})
			}
			prog := loadIn(t, cfg, dir, paths...)
			wantErrors(t, prog, dir, "")
			if len(prog.Roots) != len(paths) {
				t.Errorf("%d roots, want %d", len(prog.Roots), len(paths))
			}
			for _, p := range prog.Packages {
				if !p.Std {
					t.Errorf("package %s is not standard", p.ImportPath)
				}
			}
			if got := typeCheckProgram(t, prog, dir); got != "" {
				t.Errorf("go/types errors:\n%s", got)
			}
		})
	}
}

// testMainStd is the fake standard library with testing and its internal package testdeps.
func testMainStd() fstest.MapFS {
	std := maps.Clone(fakeStd)
	std["testing/testing.gp"] = &fstest.MapFile{Data: []byte("package testing\n\ntype T struct{}\n")}
	std["testing/internal/testdeps/deps.gp"] = &fstest.MapFile{Data: []byte("package testdeps\n\nvar ImportPath string\n")}
	return std
}

func testMainGoStd(path string) bool {
	return goStd(path) || path == "testing" || path == "testing/internal/testdeps"
}

// TestTestMain checks Config.TestMain: the generated main package is the last root, in the graph of
// the test, so it imports the test variant of the package under test and its external test package, and
// it may import testing/internal/testdeps, which a user package may not.
func TestTestMain(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":      "module example.com/m\n",
		"geom/geom.gp":   "package geom\n\nfunc Area() int { return 1 }\n",
		"geom/a_test.gp": "package geom\n\nfunc Helper() int { return Area() }\n",
		"geom/x_test.gp": "package geom_test\n\nimport \"example.com/m/geom\"\n\nvar _ = geom.Helper\n",
		"plain/plain.gp": "package plain\n",
		"user/user.gp":   "package user\n\nimport _ \"testing/internal/testdeps\"\n",
	})
	main := `package main

import (
	"testing"
	"testing/internal/testdeps"

	_test "example.com/m/geom"
	_xtest "example.com/m/geom_test"
)

var _ = testing.T{}
var _ = testdeps.ImportPath
var _ = _test.Area
var _ = _xtest.Missing

func main() {}
`
	var calls []string
	hook := func(fset *token.FileSet, p, xt *Package) ([]byte, []diag.Diagnostic) {
		calls = append(calls, fmt.Sprintf("%s xtest=%v files=%d", p.ImportPath, xt != nil, len(p.Files)))
		return []byte(main), nil
	}
	cfg := Config{Tests: true, TestMain: hook, Stdlib: testMainStd(), GoStd: testMainGoStd}
	prog := loadIn(t, cfg, root, "./geom")
	wantErrors(t, prog, root, "")
	if len(calls) != 1 || calls[0] != "example.com/m/geom xtest=true files=2" {
		t.Errorf("TestMain calls = %q", calls)
	}
	if len(prog.Roots) != 3 {
		t.Fatalf("%d roots, want the package, its external test package and the main package", len(prog.Roots))
	}
	pkg, xt, m := prog.Roots[0], prog.Roots[1], prog.Roots[2]
	if m.ImportPath != "example.com/m/geom.test" || m.Name != "main" || m.ForTest != "example.com/m/geom" || len(m.Files) != 1 {
		t.Errorf("main package = %s %s ForTest %q, %d files", m.ImportPath, m.Name, m.ForTest, len(m.Files))
	}
	if m.Files[0].Test || filepath.Base(m.Files[0].Path) != "_testmain.gp" || filepath.Dir(m.Files[0].Path) != pkg.Dir {
		t.Errorf("file of the main package = %+v", m.Files[0].Path)
	}
	if m.Imports["example.com/m/geom"] != pkg {
		t.Errorf("the main package does not import the test variant of the package")
	}
	if m.Imports["example.com/m/geom_test"] != xt {
		t.Errorf("the main package does not import the external test package")
	}
	if m.Imports["testing/internal/testdeps"] == nil || m.Imports["testing"] == nil {
		t.Errorf("imports of the main package: %v", slices.Sorted(maps.Keys(m.Imports)))
	}
	if got := typeCheckProgram(t, prog, root); !strings.Contains(got, "geom_test.Missing") && !strings.Contains(got, "Missing") {
		t.Errorf("the main package was not type-checked with the others: %q", got)
	}

	// A user package may not import testing/internal/testdeps.
	prog = loadIn(t, Config{Stdlib: testMainStd(), GoStd: testMainGoStd}, root, "./user")
	wantErrors(t, prog, root, "# example.com/m/user\nuser/user.gp:3:8: use of internal package testing/internal/testdeps not allowed\n")

	// No hook result, no main package; a package without test files is still the plain package.
	hook = func(*token.FileSet, *Package, *Package) ([]byte, []diag.Diagnostic) { return nil, nil }
	prog = loadIn(t, Config{Tests: true, TestMain: hook, Stdlib: testMainStd(), GoStd: testMainGoStd}, root, "./plain")
	if len(prog.Roots) != 1 {
		t.Errorf("%d roots without a main package", len(prog.Roots))
	}

	// Errors of the hook go to the package under test, and there is no main package.
	hook = func(fset *token.FileSet, p, xt *Package) ([]byte, []diag.Diagnostic) {
		return []byte(main), []diag.Diagnostic{{Pos: token.Position{Filename: filepath.Join(p.Dir, "a_test.gp"), Line: 3}, Msg: "wrong signature for TestX"}}
	}
	prog = loadIn(t, Config{Tests: true, TestMain: hook, Stdlib: testMainStd(), GoStd: testMainGoStd}, root, "./geom")
	if len(prog.Roots) != 2 || !prog.Roots[0].Errors.HasErrors() {
		t.Errorf("roots %d, hook errors not in the package: %v", len(prog.Roots), prog.Roots[0].Errors.Diagnostics())
	}
}
