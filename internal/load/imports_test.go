package load

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
)

// The tests parse with the stock go/parser, which accepts every import
// path; the parser fork (internal/syntax) gives the same go/ast nodes.

// importFile returns a .gp file with the given import specs, one per line
// in an import group, followed by rest. Spec i (from 0) is on line 4+i, at
// column 2, and rest starts on line 5+len(specs).
func importFile(rest string, specs ...string) string {
	var b strings.Builder
	b.WriteString("package p\n\nimport (\n")
	for _, s := range specs {
		b.WriteString("\t" + s + "\n")
	}
	b.WriteString(")\n")
	b.WriteString(rest)
	return b.String()
}

// classifyResult is what classifyFiles returns.
type classifyResult struct {
	info *FileInfo
	errs string // the errors, as diag.Printer prints them
	list *diag.List
	file *ast.File
	fset *token.FileSet
}

// classifySource parses src as the file x.gp of a package whose other files
// are more (y.gp, z.gp, ...), and classifies the imports of x.gp with the
// package-level names of all the files. gocpp package names are guessed.
func classifySource(t *testing.T, src string, more ...string) classifyResult {
	t.Helper()
	return classifyFiles(t, nil, src, more...)
}

// classifyFiles is classifySource with the real names of gocpp packages:
// pkgNames maps an import path to the name of its package. It parses with
// parser.AllErrors and fails on a parse error unless allowParseErrors is
// set, and it checks the invariants of the result (checkFileInfo).
func classifyFiles(t *testing.T, pkgNames map[string]string, src string, more ...string) classifyResult {
	t.Helper()
	return classify(t, pkgNames, false, src, more...)
}

func classify(t *testing.T, pkgNames map[string]string, allowParseErrors bool, src string, more ...string) classifyResult {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for i, s := range append([]string{src}, more...) {
		name := string(rune('x'+i)) + ".gp"
		f, err := parser.ParseFile(fset, name, s, parser.AllErrors)
		if err != nil && !allowParseErrors {
			t.Fatalf("parse %s: %v\n%s", name, err, s)
		}
		files = append(files, f)
	}
	var packageName func(string) string
	if pkgNames != nil {
		packageName = func(p string) string { return pkgNames[p] }
	}
	r := classifyResult{list: new(diag.List), file: files[0], fset: fset}
	specsBefore := len(importSpecs(r.file))
	r.info = ClassifyImports(fset, r.file, PackageLevelNames(files), packageName, r.list)
	if n := len(importSpecs(r.file)); n != specsBefore {
		t.Errorf("ClassifyImports changed the AST: %d import specs, had %d", n, specsBefore)
	}
	r.errs = printDiags(t, r.list)
	checkFileInfo(t, r.file, r.info)
	return r
}

// printDiags returns the diagnostics of l as diag.Printer prints them,
// all of them.
func printDiags(t *testing.T, l *diag.List) string {
	t.Helper()
	var b bytes.Buffer
	if err := (&diag.Printer{All: true}).Print(&b, l); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// importSpecs returns the import specs in the declarations of f, in source
// order (the tests also compare it with the parser's f.Imports).
func importSpecs(f *ast.File) []*ast.ImportSpec {
	var specs []*ast.ImportSpec
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			for _, s := range gd.Specs {
				specs = append(specs, s.(*ast.ImportSpec))
			}
		}
	}
	return specs
}

// checkFileInfo checks what must hold for every FileInfo of f: each import
// is in at most one list, in source order; Pos, Path, Spec and the name
// fields of each include agree; and the maps hold exactly the named
// includes and the renames.
func checkFileInfo(t *testing.T, f *ast.File, fi *FileInfo) {
	t.Helper()
	order := make(map[*ast.ImportSpec]int)
	for i, s := range f.Imports {
		order[s] = i
	}
	seen := make(map[*ast.ImportSpec]string)
	use := func(s *ast.ImportSpec, list string) {
		t.Helper()
		if _, ok := order[s]; !ok {
			t.Errorf("%s: spec %s is not an import of the file", list, s.Path.Value)
		}
		if other, ok := seen[s]; ok {
			t.Errorf("spec %s is in %s and in %s", s.Path.Value, other, list)
		}
		seen[s] = list
	}
	inOrder := func(list string, specs []*ast.ImportSpec) {
		t.Helper()
		if !slices.IsSortedFunc(specs, func(a, b *ast.ImportSpec) int { return order[a] - order[b] }) {
			t.Errorf("%s are not in source order", list)
		}
	}

	for _, s := range fi.Packages {
		use(s, "Packages")
	}
	inOrder("Packages", fi.Packages)

	var incSpecs []*ast.ImportSpec
	named := 0
	for _, inc := range fi.Includes {
		use(inc.Spec, "Includes")
		incSpecs = append(incSpecs, inc.Spec)
		if p, ok := importPath(inc.Spec); !ok || p != inc.Path {
			t.Errorf("include %s: Path is %q", inc.Spec.Path.Value, inc.Path)
		}
		if inc.Pos != inc.Spec.Path.Pos() {
			t.Errorf("include %s: Pos is not the position of the path", inc.Path)
		}
		if inc.Kind != SystemHeader && inc.Kind != LocalHeader {
			t.Errorf("include %s: Kind is %v", inc.Path, inc.Kind)
		}
		if inc.Name != "" {
			named++
			if fi.HeaderNames[inc.Name] != inc {
				t.Errorf("include %s: HeaderNames[%q] is not the include", inc.Path, inc.Name)
			}
			if inc.Why != HasName || inc.WouldBe != "" {
				t.Errorf("include %s has the name %s, and Why %v, WouldBe %q", inc.Path, inc.Name, inc.Why, inc.WouldBe)
			}
		} else if inc.Why == HasName {
			t.Errorf("include %s has no name, but Why is HasName", inc.Path)
		}
	}
	inOrder("Includes", incSpecs)
	if len(fi.HeaderNames) != named {
		t.Errorf("HeaderNames has %d names, but %d includes have a name", len(fi.HeaderNames), named)
	}

	for name, r := range fi.NamespaceRenames {
		use(r.Spec, "NamespaceRenames")
		if r.Name != name || r.Spec.Name == nil || r.Spec.Name.Name != name {
			t.Errorf("rename %s: Name is %q", name, r.Name)
		}
		if p, ok := importPath(r.Spec); !ok || p != r.Namespace {
			t.Errorf("rename %s: Namespace is %q", name, r.Namespace)
		}
		if r.Pos != r.Spec.Path.Pos() {
			t.Errorf("rename %s: Pos is not the position of the path", name)
		}
	}
}

// infoLines describes fi, one line per import: "pkg <name> <path>" for
// each gocpp package, then "sys"/"local" <path> <name> for each include
// (<name> is "none <Why> <WouldBe>" if it has none), then "ns <name>
// <namespace>" for each rename, sorted by name.
func infoLines(fi *FileInfo) []string {
	var lines []string
	for _, s := range fi.Packages {
		if s.Name != nil {
			lines = append(lines, "pkg "+s.Name.Name+" "+s.Path.Value)
		} else {
			lines = append(lines, "pkg "+s.Path.Value)
		}
	}
	for _, inc := range fi.Includes {
		kind := map[HeaderKind]string{SystemHeader: "sys", LocalHeader: "local"}[inc.Kind]
		if inc.Name != "" {
			lines = append(lines, fmt.Sprintf("%s %s %s", kind, inc.Path, inc.Name))
		} else {
			lines = append(lines, fmt.Sprintf("%s %s none %v %q", kind, inc.Path, inc.Why, inc.WouldBe))
		}
	}
	var renames []string
	for name, r := range fi.NamespaceRenames {
		renames = append(renames, "ns "+name+" "+r.Namespace)
	}
	slices.Sort(renames)
	return append(lines, renames...)
}

type importCase struct {
	name     string
	specs    []string          // the import specs of x.gp (see importFile)
	rest     string            // the rest of x.gp
	src      string            // all of x.gp, instead of specs and rest
	more     []string          // the other files of the package: y.gp, z.gp, ...
	pkgNames map[string]string // the real names of gocpp packages; nil: guess them
	want     []string          // infoLines of the result
	errs     string            // the errors, as diag.Printer prints them
}

func runImportCases(t *testing.T, cases []importCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.src
			if src == "" {
				src = importFile(tc.rest, tc.specs...)
			}
			r := classifyFiles(t, tc.pkgNames, src, tc.more...)
			if got := infoLines(r.info); !slices.Equal(got, tc.want) {
				t.Errorf("got:\n\t%s\nwant:\n\t%s", strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
			}
			if r.errs != tc.errs {
				t.Errorf("errors:\n%s\nwant:\n%s", r.errs, tc.errs)
			}
		})
	}
}

// TestImportTable has one case for each row of the import table in Phase 01
// README §2.3 (docs/spec/interop.md "Import forms").
func TestImportTable(t *testing.T) {
	runImportCases(t, []importCase{{
		name:  "gocpp packages",
		specs: []string{`"fmt"`, `"myapp/engine"`},
		want:  []string{`pkg "fmt"`, `pkg "myapp/engine"`},
	}, {
		name:  "system headers",
		specs: []string{`"<raylib.h>"`, `"<SDL2/SDL.h>"`, `"<cstdio>"`, `"<vector>"`},
		want:  []string{"sys <raylib.h> raylib", "sys <SDL2/SDL.h> SDL", "sys <cstdio> cstdio", "sys <vector> vector"},
	}, {
		name: "local headers, with each extension",
		specs: []string{`"./physics.h"`, `"native/x.hpp"`, `"../shared/y.h"`, `"b.hh"`, `"d.hxx"`,
			`"e.h++"`, `"f.inl"`},
		want: []string{"local ./physics.h physics", "local native/x.hpp x", "local ../shared/y.h y",
			"local b.hh b", "local d.hxx d", "local e.h++ e", "local f.inl f"},
	}, {
		name:  "a header with a chosen name, or with none",
		specs: []string{`rl "<raylib.h>"`, `_ "<GL/gl.h>"`, `phys "./physics.h"`},
		want:  []string{"sys <raylib.h> rl", `sys <GL/gl.h> none Blank "gl"`, "local ./physics.h phys"},
	}, {
		name:  "renamed namespaces",
		specs: []string{`"fmt"`, `fs "::std::filesystem"`, `cfmt "::fmt"`},
		want:  []string{`pkg "fmt"`, "ns cfmt ::fmt", "ns fs ::std::filesystem"},
	}, {
		// The example in the message renames the namespace by the last
		// part of its path, or by c and the name for a one-part path.
		name:  "namespaces without a name",
		specs: []string{`"::std"`, `"::std::chrono"`, `"::boost::range"`},
		errs: `x.gp:4:2: namespace ::std needs no import: use std.X directly, or rename it: import cstd "::std"` + "\n" +
			`x.gp:5:2: namespace ::std::chrono needs no import: use std.chrono.X directly, or rename it: import chrono "::std::chrono"` + "\n" +
			`x.gp:6:2: namespace ::boost::range needs no import: use boost.range.X directly, or rename it: import crange "::boost::range"` + "\n",
	}, {
		// Round-3 nit 2: when the first part of the path can't be a name
		// in Go code, the namespace can only be used through a rename.
		name:  "namespaces without a name that can't be used directly",
		specs: []string{`"::go"`, `"::type::x"`, `"::error"`, `"::C"`, `"::init"`, `"::_"`},
		errs: `x.gp:4:2: namespace ::go needs a new name, because go is a Go keyword: import cgo "::go"` + "\n" +
			`x.gp:5:2: namespace ::type::x needs a new name, because type is a Go keyword: import x "::type::x"` + "\n" +
			`x.gp:6:2: namespace ::error needs a new name, because error is a predeclared Go name: import cerror "::error"` + "\n" +
			`x.gp:7:2: namespace ::C needs a new name, because C is reserved for C's built-in types: import cC "::C"` + "\n" +
			`x.gp:8:2: namespace ::init needs a new name, because init can only be a func: import cinit "::init"` + "\n" +
			`x.gp:9:2: namespace ::_ needs a new name, because _ is the blank identifier: import c_ "::_"` + "\n",
	}, {
		name:  "the same namespaces with a name",
		specs: []string{`cgo "::go"`, `x "::type::x"`, `cerror "::error"`},
		want:  []string{"ns cerror ::error", "ns cgo ::go", "ns x ::type::x"},
	}, {
		// The examples of docs/spec/interop.md. Headers that are not used
		// give no "imported and not used" error.
		name:  "interop.md: a first example",
		specs: []string{`"fmt"`, ``, `"<algorithm>"`, `"<map>"`, `"<string>"`},
		want:  []string{`pkg "fmt"`, "sys <algorithm> algorithm", `sys <map> none Keyword "map"`, `sys <string> none Predeclared "string"`},
	}, {
		name:  "interop.md: namespaces are packages",
		specs: []string{`"fmt"`, ``, `fs "::std::filesystem"`, `"<chrono>"`, `"<filesystem>"`, `"<vector>"`},
		want:  []string{`pkg "fmt"`, "sys <chrono> chrono", "sys <filesystem> filesystem", "sys <vector> vector", "ns fs ::std::filesystem"},
	}, {
		name:  "interop.md: header packages and their names",
		specs: []string{`"./my-lib.h"`, `_ "<GL/gl.h>"`, `"<SDL2/SDL.h>"`, `rl "<raylib.h>"`, `cstring "<string.h>"`},
		want: []string{"local ./my-lib.h my_lib", `sys <GL/gl.h> none Blank "gl"`, "sys <SDL2/SDL.h> SDL",
			"sys <raylib.h> rl", "sys <string.h> cstring"},
	}})
}

// TestDefaultNames checks the default name of a header (decision 1 of job
// 1.6): the file name without its folder and its last extension, with
// the characters that Go doesn't allow in names turned into _, and the
// names that a header can't get.
func TestDefaultNames(t *testing.T) {
	runImportCases(t, []importCase{{
		name: "names",
		specs: []string{`"./my-lib.h"`, `"<foo.bar.h>"`, `"<über.h>"`, `"<a b.h>"`, `"<boost/version.hpp>"`,
			`"<x/y.tar.gz.h>"`, `"<cstdint>"`, `"<_private.h>"`, `"<SDL2/SDL_image.h>"`},
		want: []string{"local ./my-lib.h my_lib", "sys <foo.bar.h> foo_bar", "sys <über.h> über", "sys <a b.h> a_b",
			"sys <boost/version.hpp> version", "sys <x/y.tar.gz.h> y_tar_gz", "sys <cstdint> cstdint",
			"sys <_private.h> _private", "sys <SDL2/SDL_image.h> SDL_image"},
	}, {
		// The hint data: step 1.8 turns string.strlen into "give
		// <string.h> a name: import cstring "<string.h>"".
		name: "no name",
		specs: []string{`"<map>"`, `"<sys/select.h>"`, `"./type.h"`, `"<string>"`, `"<string.h>"`, `"<new>"`,
			`"<complex.h>"`, `"<3dmath.h>"`, `"<.h>"`, `"<SDL2/>"`, `"<C.h>"`, `"<init.h>"`, `"<_.h>"`, `"<-.h>"`},
		want: []string{
			`sys <map> none Keyword "map"`,
			`sys <sys/select.h> none Keyword "select"`,
			`local ./type.h none Keyword "type"`,
			`sys <string> none Predeclared "string"`,
			`sys <string.h> none Predeclared "string"`,
			`sys <new> none Predeclared "new"`,
			`sys <complex.h> none Predeclared "complex"`,
			`sys <3dmath.h> none NotAName "3dmath"`,
			`sys <.h> none NotAName ""`,
			`sys <SDL2/> none NotAName ""`,
			`sys <C.h> none Reserved "C"`,
			`sys <init.h> none Reserved "init"`,
			`sys <_.h> none Blank "_"`,
			`sys <-.h> none Blank "_"`,
		},
	}})
}

// TestNameUsed checks when a header doesn't get its default name because
// the name is used (decision 2 of job 1.6): by another import of the file,
// by two different headers at once, or by a package-level name.
func TestNameUsed(t *testing.T) {
	runImportCases(t, []importCase{{
		name:  "a gocpp package keeps its name",
		specs: []string{`"math"`, `"<math.h>"`},
		want:  []string{`pkg "math"`, `sys <math.h> none UsedByImport "math"`},
	}, {
		name:  "also when it comes after the header",
		specs: []string{`"<math.h>"`, `"math"`},
		want:  []string{`pkg "math"`, `sys <math.h> none UsedByImport "math"`},
	}, {
		name:  "a renamed gocpp package",
		specs: []string{`m "math"`, `"<m.h>"`, `"<math.h>"`},
		want:  []string{`pkg m "math"`, `sys <m.h> none UsedByImport "m"`, "sys <math.h> math"},
	}, {
		name:  "blank and dot imports of gocpp packages give no name",
		specs: []string{`_ "math"`, `. "strings"`, `"<math.h>"`, `"<strings.h>"`},
		want:  []string{`pkg _ "math"`, `pkg . "strings"`, "sys <math.h> math", "sys <strings.h> strings"},
	}, {
		name: "the name that the go tools assume for a gocpp package",
		specs: []string{`"math/rand/v2"`, `"gopkg.in/yaml.v3"`, `"github.com/mattn/go-sqlite3"`,
			`"<rand.h>"`, `"<yaml.h>"`, `"<sqlite3.h>"`, `"<v2.h>"`},
		want: []string{`pkg "math/rand/v2"`, `pkg "gopkg.in/yaml.v3"`, `pkg "github.com/mattn/go-sqlite3"`,
			`sys <rand.h> none UsedByImport "rand"`, `sys <yaml.h> none UsedByImport "yaml"`,
			`sys <sqlite3.h> none UsedByImport "sqlite3"`, "sys <v2.h> v2"},
	}, {
		name:  "a name chosen for another header",
		specs: []string{`"<rl.h>"`, `rl "./wrap.h"`},
		want:  []string{`sys <rl.h> none UsedByImport "rl"`, "local ./wrap.h rl"},
	}, {
		name:  "a namespace rename",
		specs: []string{`"<fs.h>"`, `fs "::std::filesystem"`},
		want:  []string{`sys <fs.h> none UsedByImport "fs"`, "ns fs ::std::filesystem"},
	}, {
		name:  "different headers with the same default name: none gets it",
		specs: []string{`"<time.h>"`, `"<sys/time.h>"`, `"./util/time.hpp"`},
		want: []string{`sys <time.h> none UsedByImport "time"`, `sys <sys/time.h> none UsedByImport "time"`,
			`local ./util/time.hpp none UsedByImport "time"`},
	}, {
		name:  "a blank header import doesn't use its name",
		specs: []string{`_ "<GL/gl.h>"`, `"<GLES/gl.h>"`},
		want:  []string{`sys <GL/gl.h> none Blank "gl"`, "sys <GLES/gl.h> gl"},
	}, {
		name:  "a package-level func",
		specs: []string{`"<raylib.h>"`},
		rest:  "\nfunc raylib() {}\n",
		want:  []string{`sys <raylib.h> none UsedByPackage "raylib"`},
	}, {
		name:  "package-level types, consts and vars",
		specs: []string{`"<vector>"`, `"<cmath>"`, `"<cstdio>"`},
		rest:  "\ntype vector []int\n\nconst cmath = 1\n\nvar _, cstdio = 1, 2\n",
		want: []string{`sys <vector> none UsedByPackage "vector"`, `sys <cmath> none UsedByPackage "cmath"`,
			`sys <cstdio> none UsedByPackage "cstdio"`},
	}, {
		name:  "a package-level name in another file",
		specs: []string{`"<raylib.h>"`},
		more:  []string{"package p\n\nvar raylib = 1\n"},
		want:  []string{`sys <raylib.h> none UsedByPackage "raylib"`},
	}, {
		name:  "func main is a package-level name",
		specs: []string{`"<main.h>"`},
		rest:  "\nfunc main() {}\n",
		want:  []string{`sys <main.h> none UsedByPackage "main"`},
	}, {
		name:  "methods and locals are not package-level names",
		specs: []string{`"<raylib.h>"`, `"<cstdio>"`},
		rest:  "\ntype T int\n\nfunc (T) raylib() {}\n\nfunc f() {\n\tcstdio := 1\n\t_ = cstdio\n}\n",
		want:  []string{"sys <raylib.h> raylib", "sys <cstdio> cstdio"},
	}, {
		name:  "an import and a package-level name",
		specs: []string{`"time"`, `"<time.h>"`, `"<raylib.h>"`},
		rest:  "\nvar time, raylib int\n",
		want:  []string{`pkg "time"`, `sys <time.h> none UsedByImport "time"`, `sys <raylib.h> none UsedByPackage "raylib"`},
	}})
}

// TestSameHeaderTwice checks a header imported twice with the same name,
// chosen or default (round-2 decision 2 of job 1.6 and the decision after
// it): like Go's import "fmt" twice, the second import is an error, and
// the first one keeps the name.
func TestSameHeaderTwice(t *testing.T) {
	runImportCases(t, []importCase{{
		name:  "twice",
		specs: []string{`"<raylib.h>"`, `"<raylib.h>"`},
		want:  []string{"sys <raylib.h> raylib", `sys <raylib.h> none UsedByImport "raylib"`},
		errs:  "x.gp:5:2: raylib redeclared in this block\n\tx.gp:4:2: other declaration of raylib\n",
	}, {
		name:  "three times",
		specs: []string{`"./physics.h"`, `"<vector>"`, `"./physics.h"`, `"./physics.h"`},
		want: []string{"local ./physics.h physics", "sys <vector> vector",
			`local ./physics.h none UsedByImport "physics"`, `local ./physics.h none UsedByImport "physics"`},
		errs: "x.gp:6:2: physics redeclared in this block\n\tx.gp:4:2: other declaration of physics\n" +
			"x.gp:7:2: physics redeclared in this block\n\tx.gp:4:2: other declaration of physics\n",
	}, {
		name:  "and a different header with the same name: none gets it, no error",
		specs: []string{`"<time.h>"`, `"<time.h>"`, `"<sys/time.h>"`},
		want: []string{`sys <time.h> none UsedByImport "time"`, `sys <time.h> none UsedByImport "time"`,
			`sys <sys/time.h> none UsedByImport "time"`},
	}, {
		name:  "two spellings of one file are different paths",
		specs: []string{`"./physics.h"`, `"physics.h"`},
		want:  []string{`local ./physics.h none UsedByImport "physics"`, `local physics.h none UsedByImport "physics"`},
	}, {
		name:  "a name that another import uses: no error",
		specs: []string{`"math"`, `"<math.h>"`, `"<math.h>"`},
		want:  []string{`pkg "math"`, `sys <math.h> none UsedByImport "math"`, `sys <math.h> none UsedByImport "math"`},
	}, {
		name:  "a package-level name: no error",
		specs: []string{`"<raylib.h>"`, `"<raylib.h>"`},
		rest:  "\nfunc raylib() {}\n",
		want:  []string{`sys <raylib.h> none UsedByPackage "raylib"`, `sys <raylib.h> none UsedByPackage "raylib"`},
	}, {
		// The manager's decision after round 2: a name chosen for the
		// same header is a second declaration, like import "fmt" and
		// import fmt "fmt" in Go; the first import keeps the name.
		name:  "a default name, then the same name chosen",
		specs: []string{`"<raylib.h>"`, `raylib "<raylib.h>"`},
		want:  []string{"sys <raylib.h> raylib", `sys <raylib.h> none UsedByImport "raylib"`},
		errs:  "x.gp:5:2: raylib redeclared in this block\n\tx.gp:4:2: other declaration of raylib\n",
	}, {
		name:  "the chosen name first",
		specs: []string{`raylib "<raylib.h>"`, `"<raylib.h>"`},
		want:  []string{"sys <raylib.h> raylib", `sys <raylib.h> none UsedByImport "raylib"`},
		errs:  "x.gp:5:2: raylib redeclared in this block\n\tx.gp:4:2: other declaration of raylib\n",
	}, {
		name:  "the chosen name, then the default name twice",
		specs: []string{`raylib "<raylib.h>"`, `"<raylib.h>"`, `"<raylib.h>"`},
		want: []string{"sys <raylib.h> raylib", `sys <raylib.h> none UsedByImport "raylib"`,
			`sys <raylib.h> none UsedByImport "raylib"`},
		errs: "x.gp:5:2: raylib redeclared in this block\n\tx.gp:4:2: other declaration of raylib\n" +
			"x.gp:6:2: raylib redeclared in this block\n\tx.gp:4:2: other declaration of raylib\n",
	}, {
		name:  "a different name chosen: two names for one header",
		specs: []string{`"<raylib.h>"`, `rl "<raylib.h>"`},
		want:  []string{"sys <raylib.h> raylib", "sys <raylib.h> rl"},
	}, {
		name:  "a different header with the same default name: the default names give way",
		specs: []string{`"<raylib.h>"`, `raylib "<raylib.h>"`, `"./raylib.h"`},
		want: []string{`sys <raylib.h> none UsedByImport "raylib"`, "sys <raylib.h> raylib",
			`local ./raylib.h none UsedByImport "raylib"`},
	}, {
		name:  "a package-level name: the default name gives way, the chosen one is an error",
		specs: []string{`"<raylib.h>"`, `raylib "<raylib.h>"`},
		rest:  "\nvar raylib = 1\n",
		want:  []string{`sys <raylib.h> none UsedByPackage "raylib"`, "sys <raylib.h> raylib"},
		errs:  "x.gp:8:5: raylib already declared through import of <raylib.h>\n\tx.gp:5:2: other declaration of raylib\n",
	}})
}

// TestDefaultNamesOrder checks that the names of headers don't depend on
// the order of the imports (decision 2 of job 1.6), with every order of
// seven imports.
func TestDefaultNamesOrder(t *testing.T) {
	specs := []string{`"math"`, `"<math.h>"`, `"<time.h>"`, `"<sys/time.h>"`, `rl "<raylib.h>"`, `"<rl.h>"`, `"<SDL2/SDL.h>"`}
	want := map[string]string{
		"<math.h>":     `none UsedByImport "math"`,
		"<time.h>":     `none UsedByImport "time"`,
		"<sys/time.h>": `none UsedByImport "time"`,
		"<raylib.h>":   "rl",
		"<rl.h>":       `none UsedByImport "rl"`,
		"<SDL2/SDL.h>": "SDL",
	}
	orders := make(map[string]bool)
	permute(specs, func(order []string) {
		orders[strings.Join(order, ";")] = true
		r := classifySource(t, importFile("", order...))
		got := make(map[string]string)
		for _, inc := range r.info.Includes {
			if inc.Name != "" {
				got[inc.Path] = inc.Name
			} else {
				got[inc.Path] = fmt.Sprintf("none %v %q", inc.Why, inc.WouldBe)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) || r.errs != "" || len(r.info.Packages) != 1 {
			t.Fatalf("imports %v:\ngot  %v, %d packages, errors %q\nwant %v", order, got, len(r.info.Packages), r.errs, want)
		}
	})
	if len(orders) != 5040 {
		t.Errorf("checked %d different orders, want 7! = 5040", len(orders))
	}
}

// permute calls f with every order of s (Heap's algorithm). f must not
// keep the slice.
func permute(s []string, f func([]string)) {
	s = slices.Clone(s)
	var gen func(k int)
	gen = func(k int) {
		if k <= 1 {
			f(s)
			return
		}
		for i := range k - 1 {
			gen(k - 1)
			if k%2 == 0 {
				s[i], s[k-1] = s[k-1], s[i]
			} else {
				s[0], s[k-1] = s[k-1], s[0]
			}
		}
		gen(k - 1)
	}
	gen(len(s))
}

// TestChosenNames checks names given in imports (decision 3 of job 1.6):
// they follow Go, with go/types' messages.
func TestChosenNames(t *testing.T) {
	runImportCases(t, []importCase{{
		name:  "two headers",
		specs: []string{`rl "<raylib.h>"`, `rl "./rl.h"`},
		want:  []string{"sys <raylib.h> rl", `local ./rl.h none UsedByImport "rl"`},
		errs:  "x.gp:5:2: rl redeclared in this block\n\tx.gp:4:2: other declaration of rl\n",
	}, {
		name:  "a header named like a gocpp package",
		specs: []string{`"math"`, `math "<math.h>"`},
		want:  []string{`pkg "math"`, `sys <math.h> none UsedByImport "math"`},
		errs:  "x.gp:5:2: math redeclared in this block\n\tx.gp:4:2: other declaration of math\n",
	}, {
		name:  "a gocpp package named like a header",
		specs: []string{`rl "<raylib.h>"`, `rl "fmt"`},
		want:  []string{`pkg rl "fmt"`, "sys <raylib.h> rl"},
		errs:  "x.gp:5:2: rl redeclared in this block\n\tx.gp:4:2: other declaration of rl\n",
	}, {
		name:  "a gocpp package after a header with its name",
		specs: []string{`fmt "./fmtx.h"`, `"fmt"`},
		want:  []string{`pkg "fmt"`, "local ./fmtx.h fmt"},
		errs:  "x.gp:5:2: fmt redeclared in this block\n\tx.gp:4:2: other declaration of fmt\n",
	}, {
		name:  "a header and a namespace",
		specs: []string{`fs "<fs.h>"`, `fs "::std::filesystem"`},
		want:  []string{"sys <fs.h> fs"},
		errs:  "x.gp:5:2: fs redeclared in this block\n\tx.gp:4:2: other declaration of fs\n",
	}, {
		name:  "two namespaces",
		specs: []string{`fs "::std::filesystem"`, `fs "::boost::filesystem"`},
		want:  []string{"ns fs ::std::filesystem"},
		errs:  "x.gp:5:2: fs redeclared in this block\n\tx.gp:4:2: other declaration of fs\n",
	}, {
		name:  "three headers",
		specs: []string{`rl "<a.h>"`, `rl "<b.h>"`, `rl "<c.h>"`},
		want:  []string{"sys <a.h> rl", `sys <b.h> none UsedByImport "rl"`, `sys <c.h> none UsedByImport "rl"`},
		errs: "x.gp:5:2: rl redeclared in this block\n\tx.gp:4:2: other declaration of rl\n" +
			"x.gp:6:2: rl redeclared in this block\n\tx.gp:4:2: other declaration of rl\n",
	}, {
		name:  "two gocpp packages: go/types reports it",
		specs: []string{`"fmt"`, `fmt "os"`},
		want:  []string{`pkg "fmt"`, `pkg fmt "os"`},
	}, {
		name:  "the same header under two names",
		specs: []string{`rl "<raylib.h>"`, `ray "<raylib.h>"`},
		want:  []string{"sys <raylib.h> rl", "sys <raylib.h> ray"},
	}, {
		name:  "a chosen name wins over a default name",
		specs: []string{`"<raylib.h>"`, `raylib "./myraylib.h"`},
		want:  []string{`sys <raylib.h> none UsedByImport "raylib"`, "local ./myraylib.h raylib"},
	}, {
		// Round-2 decision 3: go/types never sees a header or namespace
		// import, so it couldn't hide the predeclared name in the file.
		name:  "predeclared names can't be chosen for headers",
		specs: []string{`string "<string.h>"`, `nil "<n.h>"`, `true "<t.h>"`, `len "./len.h"`},
		want: []string{`sys <string.h> none Predeclared "string"`, `sys <n.h> none Predeclared "nil"`,
			`sys <t.h> none Predeclared "true"`, `local ./len.h none Predeclared "len"`},
		errs: "x.gp:4:2: cannot import header as string - string is a predeclared Go name (use another name, like cstring)\n" +
			"x.gp:5:2: cannot import header as nil - nil is a predeclared Go name (use another name, like cnil)\n" +
			"x.gp:6:2: cannot import header as true - true is a predeclared Go name (use another name, like ctrue)\n" +
			"x.gp:7:2: cannot import header as len - len is a predeclared Go name (use another name, like clen)\n",
	}, {
		name:  "or for namespaces",
		specs: []string{`string "::std::string"`},
		errs:  "x.gp:4:2: cannot import namespace as string - string is a predeclared Go name (use another name, like cstring)\n",
	}, {
		name:  "but for gocpp packages, like in Go",
		specs: []string{`string "strings"`},
		want:  []string{`pkg string "strings"`},
	}, {
		name:  "a header name that is a package-level name",
		specs: []string{`rl "<raylib.h>"`},
		rest:  "\nvar rl = 1\n",
		want:  []string{"sys <raylib.h> rl"},
		errs:  "x.gp:7:5: rl already declared through import of <raylib.h>\n\tx.gp:4:2: other declaration of rl\n",
	}, {
		name:  "a package-level name in another file",
		specs: []string{`rl "<raylib.h>"`},
		more:  []string{"package p\n\nfunc rl() {}\n"},
		want:  []string{"sys <raylib.h> rl"},
		errs:  "y.gp:3:6: rl already declared through import of <raylib.h>\n\tx.gp:4:2: other declaration of rl\n",
	}, {
		name:  "a local header",
		specs: []string{`phys "./physics.h"`},
		rest:  "\nconst phys = 1\n",
		want:  []string{"local ./physics.h phys"},
		errs:  "x.gp:7:7: phys already declared through import of \"./physics.h\"\n\tx.gp:4:2: other declaration of phys\n",
	}, {
		name:  "a namespace rename",
		specs: []string{`fs "::std::filesystem"`},
		rest:  "\ntype fs struct{}\n",
		want:  []string{"ns fs ::std::filesystem"},
		errs:  "x.gp:7:6: fs already declared through import of ::std::filesystem\n\tx.gp:4:2: other declaration of fs\n",
	}, {
		name:  "redeclared and a package-level name: one error",
		specs: []string{`"math"`, `math "<math.h>"`},
		rest:  "\nvar math = 1\n",
		want:  []string{`pkg "math"`, `sys <math.h> none UsedByImport "math"`},
		errs:  "x.gp:5:2: math redeclared in this block\n\tx.gp:4:2: other declaration of math\n",
	}})
}

// TestPackageLevelNoPos checks a package-level name without a position:
// the error is at the import.
func TestPackageLevelNoPos(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", importFile("", `rl "<raylib.h>"`, `fs "::std::filesystem"`), 0)
	if err != nil {
		t.Fatal(err)
	}
	var l diag.List
	fi := ClassifyImports(fset, f, map[string]token.Pos{"rl": token.NoPos, "fs": token.NoPos}, nil, &l)
	want := "x.gp:4:2: rl already declared through import of <raylib.h>\n" +
		"x.gp:5:2: fs already declared through import of ::std::filesystem\n"
	if got := printDiags(t, &l); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	if fi.HeaderNames["rl"] == nil || fi.NamespaceRenames["fs"] == nil {
		t.Errorf("the names are not kept: %v", infoLines(fi))
	}
}

// TestRealPackageNames checks the names of gocpp packages that the caller
// passes (round-2 decision 1 of job 1.6), where the guess from the path
// would be wrong, and the guess when the caller knows no name.
func TestRealPackageNames(t *testing.T) {
	runImportCases(t, []importCase{{
		name:     "myapp/api/v1 holds package v1",
		specs:    []string{`"myapp/api/v1"`, `api "./api.h"`, `"<v1.h>"`},
		pkgNames: map[string]string{"myapp/api/v1": "v1"},
		want:     []string{`pkg "myapp/api/v1"`, "local ./api.h api", `sys <v1.h> none UsedByImport "v1"`},
	}, {
		name:  "the same, guessed: api",
		specs: []string{`"myapp/api/v1"`, `api "./api.h"`, `"<v1.h>"`},
		want:  []string{`pkg "myapp/api/v1"`, `local ./api.h none UsedByImport "api"`, "sys <v1.h> v1"},
		errs:  "x.gp:5:2: api redeclared in this block\n\tx.gp:4:2: other declaration of api\n",
	}, {
		name:     "myapp/my-engine holds package engine",
		specs:    []string{`"myapp/my-engine"`, `"<engine.h>"`, `"<my.h>"`},
		pkgNames: map[string]string{"myapp/my-engine": "engine"},
		want:     []string{`pkg "myapp/my-engine"`, `sys <engine.h> none UsedByImport "engine"`, "sys <my.h> my"},
	}, {
		name:  "the same, guessed: my",
		specs: []string{`"myapp/my-engine"`, `"<engine.h>"`, `"<my.h>"`},
		want:  []string{`pkg "myapp/my-engine"`, "sys <engine.h> engine", `sys <my.h> none UsedByImport "my"`},
	}, {
		name:     "a path without a known name is guessed",
		specs:    []string{`"math"`, `"<math.h>"`},
		pkgNames: map[string]string{"other": "x"},
		want:     []string{`pkg "math"`, `sys <math.h> none UsedByImport "math"`},
	}, {
		// Finding 4 of round 1: a package named C would bring a second C
		// into the file.
		name:     "a package named C",
		specs:    []string{`"myapp/cdefs"`, `c "myapp/cdefs"`},
		pkgNames: map[string]string{"myapp/cdefs": "C"},
		want:     []string{`pkg c "myapp/cdefs"`},
		errs:     `x.gp:4:2: cannot import package as C - C is reserved for C's built-in types (give it another name, like import c "myapp/cdefs")` + "\n",
	}, {
		name:  "a package guessed to be named C",
		specs: []string{`"myapp/C"`},
		errs:  `x.gp:4:2: cannot import package as C - C is reserved for C's built-in types (give it another name, like import c "myapp/C")` + "\n",
	}})
}

// TestPackageNameCalls checks that ClassifyImports asks for the name of a
// gocpp package only when the import doesn't give one.
func TestPackageNameCalls(t *testing.T) {
	fset := token.NewFileSet()
	src := importFile("", `"fmt"`, `f "os"`, `_ "strings"`, `. "bytes"`, `"<x.h>"`, `"./y.h"`, `x "::std"`, `"myapp/a"`)
	f, err := parser.ParseFile(fset, "x.gp", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	var l diag.List
	ClassifyImports(fset, f, nil, func(p string) string {
		calls = append(calls, p)
		return ""
	}, &l)
	if want := []string{"fmt", "myapp/a"}; !slices.Equal(calls, want) {
		t.Errorf("packageName was called with %q, want %q", calls, want)
	}
}

// TestBadImports checks the imports that are errors (decision 4 of job
// 1.6, and round-2 findings 8 and 9). A bad path makes the import go away;
// a header with a bad name is still included, with no name.
func TestBadImports(t *testing.T) {
	const noImportC = `import "C" is not allowed: C's built-in types need no import in gocpp, ` +
		`and there is no inline C preamble (put C code in a header next to the .gp file)`
	const compiled = `: a C or C++ file can't be imported (import its header; gocpp compiles the C and C++ files in the package folder)`
	const notHeader = `: a local header ends in .h, .hh, .hpp, .hxx, .h++ or .inl`
	runImportCases(t, []importCase{
		// Paths.
		{name: "empty path", specs: []string{`""`}, errs: "x.gp:4:2: invalid import path (empty string)\n"},
		{name: "<>", specs: []string{`"<>"`}, errs: `x.gp:4:2: invalid header path "<>": missing file name` + "\n"},
		{name: "no >", specs: []string{`"<raylib.h"`}, errs: `x.gp:4:2: invalid header path "<raylib.h": missing > at the end` + "\n"},
		{name: "no <", specs: []string{`"raylib.h>"`}, errs: `x.gp:4:2: invalid header path "raylib.h>": missing < at the start` + "\n"},
		{name: "<", specs: []string{`"<"`}, errs: `x.gp:4:2: invalid header path "<": missing > at the end` + "\n"},
		{name: ">", specs: []string{`">"`}, errs: `x.gp:4:2: invalid header path ">": missing < at the start` + "\n"},
		{name: "named, no >", specs: []string{`rl "<raylib.h"`}, errs: `x.gp:4:5: invalid header path "<raylib.h": missing > at the end` + "\n"},
		{name: `\ in a system header`, specs: []string{`"<GL\\gl.h>"`}, errs: `x.gp:4:2: invalid header path "<GL\\gl.h>": use / in header paths` + "\n"},
		{name: `\ in a local header`, specs: []string{"`native\\x.hpp`"}, errs: `x.gp:4:2: invalid header path "native\\x.hpp": use / in header paths` + "\n"},
		{name: "newline", specs: []string{`"<a\nb.h>"`}, errs: `x.gp:4:2: invalid header path "<a\nb.h>": invalid character U+000A` + "\n"},
		{name: "tab", specs: []string{`"./a\tb.h"`}, errs: `x.gp:4:2: invalid header path "./a\tb.h": invalid character U+0009` + "\n"},
		{name: "bidi control", specs: []string{`"<a\u202eb.h>"`}, errs: `x.gp:4:2: invalid header path "<a\u202eb.h>": invalid character U+202E` + "\n"},
		{name: "not UTF-8", specs: []string{`"<a\xffb.h>"`}, errs: `x.gp:4:2: invalid header path "<a\xffb.h>": invalid character U+FFFD '�'` + "\n"},
		{name: "> in a system header", specs: []string{`"<a>b.h>"`}, errs: `x.gp:4:2: invalid header path "<a>b.h>": invalid character U+003E '>'` + "\n"},
		{name: "< in a system header", specs: []string{`"<<raylib.h>"`}, errs: `x.gp:4:2: invalid header path "<<raylib.h>": invalid character U+003C '<'` + "\n"},
		{name: `" in a local header`, specs: []string{`"./a\"b.h"`}, errs: `x.gp:4:2: invalid header path "./a\"b.h": invalid character U+0022 '"'` + "\n"},
		{name: "::", specs: []string{`fs "::"`}, errs: `x.gp:4:5: invalid namespace path "::": missing name after ::` + "\n"},
		{name: "::std::", specs: []string{`fs "::std::"`}, errs: `x.gp:4:5: invalid namespace path "::std::": missing name after ::` + "\n"},
		{name: "::std::::chrono", specs: []string{`ch "::std::::chrono"`}, errs: `x.gp:4:5: invalid namespace path "::std::::chrono": missing name after ::` + "\n"},
		{name: "::1x", specs: []string{`x "::1x"`}, errs: `x.gp:4:4: invalid namespace path "::1x": "1x" is not a C++ name` + "\n"},
		{name: "template", specs: []string{`v "::std::vector<int>"`}, errs: `x.gp:4:4: invalid namespace path "::std::vector<int>": "vector<int>" is not a C++ name` + "\n"},
		{name: "space inside", specs: []string{`s "::std:: chrono"`}, errs: `x.gp:4:4: invalid namespace path "::std:: chrono": " chrono" is not a C++ name` + "\n"},
		{name: "no leading ::", specs: []string{`v "std::vector"`}, errs: `x.gp:4:4: invalid namespace path "std::vector": it must start with ::, like "::std::vector"` + "\n"},
		{name: "no leading ::, no name", specs: []string{`"std::vector"`}, errs: `x.gp:4:2: invalid namespace path "std::vector": it must start with ::, like "::std::vector"` + "\n"},
		{name: "no leading :: and more", specs: []string{`s "std::"`}, errs: `x.gp:4:4: invalid namespace path "std::": it must start with ::` + "\n"},

		// Spaces at the ends (finding 8).
		{
			name: "spaces at the ends",
			specs: []string{`" <raylib.h>"`, `"<raylib.h> "`, `"<raylib.h>\t"`, `"<raylib.h >"`, `"< raylib.h>"`,
				`"<   >"`, `" ./x.h"`, `"./x.h "`, `" ::std"`, `fs "::std "`, `" ./physics"`},
			errs: `x.gp:4:2: invalid header path " <raylib.h>": space at the start` + "\n" +
				`x.gp:5:2: invalid header path "<raylib.h> ": space at the end` + "\n" +
				`x.gp:6:2: invalid header path "<raylib.h>\t": space at the end` + "\n" +
				`x.gp:7:2: invalid header path "<raylib.h >": space before >` + "\n" +
				`x.gp:8:2: invalid header path "< raylib.h>": space after <` + "\n" +
				`x.gp:9:2: invalid header path "<   >": missing file name` + "\n" +
				`x.gp:10:2: invalid header path " ./x.h": space at the start` + "\n" +
				`x.gp:11:2: invalid header path "./x.h ": space at the end` + "\n" +
				`x.gp:12:2: invalid namespace path " ::std": space at the start` + "\n" +
				`x.gp:13:5: invalid namespace path "::std ": space at the end` + "\n" +
				`x.gp:14:2: invalid import path " ./physics": space at the start` + "\n",
		},
		{name: "a space around a gocpp path is go/types' error", specs: []string{`" fmt"`}, want: []string{`pkg " fmt"`}},

		// ./ and ../ paths that are not headers (finding 9).
		{
			name:  "C and C++ files",
			specs: []string{`"./helper.cpp"`, `"../src/x.c"`, `"./a.cc"`, `"./b.cxx"`, `"./sub/x.c"`},
			errs: `x.gp:4:2: cannot import "./helper.cpp"` + compiled + "\n" +
				`x.gp:5:2: cannot import "../src/x.c"` + compiled + "\n" +
				`x.gp:6:2: cannot import "./a.cc"` + compiled + "\n" +
				`x.gp:7:2: cannot import "./b.cxx"` + compiled + "\n" +
				`x.gp:8:2: cannot import "./sub/x.c"` + compiled + "\n",
		},
		{
			name:  "other files",
			specs: []string{`"./x.H"`, `"./x.HPP"`, `"./physics"`, `"../shared"`},
			errs: `x.gp:4:2: invalid import path "./x.H"` + notHeader + "\n" +
				`x.gp:5:2: invalid import path "./x.HPP"` + notHeader + "\n" +
				`x.gp:6:2: invalid import path "./physics"` + notHeader + "\n" +
				`x.gp:7:2: invalid import path "../shared"` + notHeader + "\n",
		},
		{name: "a path without ./ stays a gocpp package", specs: []string{`"x.c"`, `"lib/x.cpp"`}, want: []string{`pkg "x.c"`, `pkg "lib/x.cpp"`}},

		// Names.
		{
			name:  "dot-imported header",
			specs: []string{`. "<raylib.h>"`},
			want:  []string{`sys <raylib.h> none DotImport "raylib"`},
			errs:  "x.gp:4:2: cannot dot-import a header\n",
		},
		{name: "dot-imported namespace", specs: []string{`. "::std"`}, errs: "x.gp:4:2: cannot dot-import a namespace\n"},
		{name: "blank namespace", specs: []string{`_ "::std"`}, errs: "x.gp:4:2: cannot import namespace as _ - a namespace import only gives it a new name\n"},
		{
			name:  "header named C",
			specs: []string{`C "<raylib.h>"`, `C "./physics.h"`},
			want:  []string{`sys <raylib.h> none Reserved "C"`, `local ./physics.h none Reserved "C"`},
			errs: "x.gp:4:2: cannot import header as C - C is reserved for C's built-in types\n" +
				"x.gp:5:2: cannot import header as C - C is reserved for C's built-in types\n",
		},
		{name: "namespace named C", specs: []string{`C "::std"`}, errs: "x.gp:4:2: cannot import namespace as C - C is reserved for C's built-in types\n"},
		{name: "gocpp package named C", specs: []string{`C "fmt"`}, errs: "x.gp:4:2: cannot import package as C - C is reserved for C's built-in types\n"},
		{
			name:  "header named init",
			specs: []string{`init "<raylib.h>"`},
			want:  []string{`sys <raylib.h> none Reserved "init"`},
			errs:  "x.gp:4:2: cannot import header as init - init must be a func\n",
		},
		{name: "namespace named init", specs: []string{`init "::std"`}, errs: "x.gp:4:2: cannot import namespace as init - init must be a func\n"},
		{name: "gocpp package named init: go/types reports it", specs: []string{`init "fmt"`}, want: []string{`pkg init "fmt"`}},
		{
			name:  `import "C"`,
			specs: []string{`"C"`, `x "C"`, `_ "C"`, `. "C"`, `C "C"`},
			errs: "x.gp:4:2: " + noImportC + "\n" + "x.gp:5:4: " + noImportC + "\n" + "x.gp:6:4: " + noImportC + "\n" +
				"x.gp:7:4: " + noImportC + "\n" + "x.gp:8:4: " + noImportC + "\n",
		},
		{
			name: `import "C" with a cgo preamble`,
			src:  "package p\n\n// #include <stdio.h>\nimport \"C\"\n",
			errs: "x.gp:4:8: " + noImportC + "\n",
		},
		{
			name:  "gocpp imports that go/types checks",
			specs: []string{`_ "fmt"`, `. "fmt"`, `"a b"`},
			want:  []string{`pkg _ "fmt"`, `pkg . "fmt"`, `pkg "a b"`},
		},
		{
			name:  "a bad import doesn't change the others",
			specs: []string{`"<raylib.h"`, `"<vector>"`, `"::std"`, `"math"`},
			want:  []string{`pkg "math"`, "sys <vector> vector"},
			errs: `x.gp:4:2: invalid header path "<raylib.h": missing > at the end` + "\n" +
				`x.gp:6:2: namespace ::std needs no import: use std.X directly, or rename it: import cstd "::std"` + "\n",
		},
	})
}

// TestBrokenFiles classifies files with syntax errors, as the language
// server does all the time (finding 5 of round 1). ClassifyImports and
// CheckReservedC must not panic, an import whose path the parser couldn't
// read is skipped without an error (the parser reported it), and every
// other import that the parser kept is classified.
func TestBrokenFiles(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // infoLines
		errs string   // the errors of ClassifyImports
	}{
		{name: "a number as the path", src: "package p\n\nimport 42\nimport \"fmt\"\n", want: []string{`pkg "fmt"`}},
		{name: "a name without a path", src: "package p\n\nimport rl\nimport \"<raylib.h>\"\n", want: []string{"sys <raylib.h> raylib"}},
		{name: "a dot without a path", src: "package p\n\nimport .\nimport \"<vector>\"\n"},
		{name: "an unterminated string", src: "package p\n\nimport \"fmt\nimport \"<vector>\"\n", want: []string{"sys <vector> vector"}},
		{name: "a bad escape", src: "package p\n\nimport \"<GL\\gl.h>\"\nimport \"<vector>\"\n", want: []string{"sys <vector> vector"}},
		{
			name: "an unclosed import group",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\trl \"<raylib.h>\"\n",
			want: []string{`pkg "fmt"`, "sys <raylib.h> rl"},
		},
		{name: "a keyword as the name", src: "package p\n\nimport map \"<map>\"\nimport \"<vector>\"\n", want: []string{"sys <vector> vector"}},
		{name: "a group on one line", src: "package p\n\nimport ( \"fmt\" \"<vector>\" )\n", want: []string{`pkg "fmt"`}},
		{
			name: "imports after other declarations",
			src:  "package p\n\nvar x = 1\n\nimport \"fmt\"\nimport C \"<raylib.h>\"\n",
			want: []string{`pkg "fmt"`, `sys <raylib.h> none Reserved "C"`},
			errs: "x.gp:6:8: cannot import header as C - C is reserved for C's built-in types\n",
		},
		{
			name: "broken declarations",
			src:  "package p\n\nimport \"<cstdio>\"\n\nfunc (C int\nvar C =\nfunc f() { C := }\ntype C[\n",
			want: []string{"sys <cstdio> cstdio"},
		},
		{name: "an import inside a func", src: "package p\n\nfunc f() {\n\timport \"fmt\"\n\tC := 1\n}\n"},
		{name: "a func without its end", src: "package p\n\nfunc C(\n"},
		{name: "no imports", src: "package p\n\nfunc main() {}\n"},
		{name: "only a package clause", src: "package p"},
		{name: "an empty file", src: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := classify(t, nil, true, tc.src)
			if got := infoLines(r.info); !slices.Equal(got, tc.want) {
				t.Errorf("got:\n\t%s\nwant:\n\t%s", strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
			}
			if r.errs != tc.errs {
				t.Errorf("errors:\n%s\nwant:\n%s", r.errs, tc.errs)
			}
			if !slices.Equal(fileImports(r.file), r.file.Imports) {
				t.Errorf("fileImports differs from the parser's f.Imports")
			}

			// Every import with a readable path is in a list or has an
			// error on its line.
			placed := make(map[*ast.ImportSpec]bool)
			for _, s := range r.info.Packages {
				placed[s] = true
			}
			for _, inc := range r.info.Includes {
				placed[inc.Spec] = true
			}
			for _, rn := range r.info.NamespaceRenames {
				placed[rn.Spec] = true
			}
			errLines := make(map[int]bool)
			for _, d := range r.list.Diagnostics() {
				errLines[d.Pos.Line] = true
			}
			for _, s := range r.file.Imports {
				line := r.fset.Position(s.Path.Pos()).Line
				if _, ok := importPath(s); ok && !placed[s] && !errLines[line] {
					t.Errorf("import %s on line %d is not classified", s.Path.Value, line)
				}
			}

			var reserved diag.List
			CheckReservedC(r.fset, r.file, &reserved)
			for _, d := range reserved.Diagnostics() {
				if !d.Pos.IsValid() || d.Msg != reservedC {
					t.Errorf("CheckReservedC: %s: %s", d.Pos, d.Msg)
				}
			}
		})
	}
}

// TestImportPositions checks the order of the includes, the specs they come
// from, the positions they keep, and the end positions of the errors.
func TestImportPositions(t *testing.T) {
	const src = `package p

import "fmt"

import (
	rl "<raylib.h>"
	"./physics.h"
	fs "::std::filesystem"
	"<vector>"
	_ "<GL/gl.h>"
	. "<x.h>"
)

import "<cstdio>"
`
	r := classifySource(t, src)
	specs := importSpecs(r.file)
	want := []struct {
		spec int // the index in specs
		kind HeaderKind
		name string
	}{{1, SystemHeader, "rl"}, {2, LocalHeader, "physics"}, {4, SystemHeader, "vector"}, {5, SystemHeader, ""}, {6, SystemHeader, ""}, {7, SystemHeader, "cstdio"}}
	if len(r.info.Includes) != len(want) {
		t.Fatalf("got %d includes, want %d: %v", len(r.info.Includes), len(want), infoLines(r.info))
	}
	for i, w := range want {
		inc := r.info.Includes[i]
		if inc.Spec != specs[w.spec] || inc.Kind != w.kind || inc.Name != w.name {
			t.Errorf("include %d is %s %s %q, want spec %d, %s %q", i, inc.Spec.Path.Value, inc.Kind, inc.Name, w.spec, w.kind, w.name)
		}
	}
	if got := r.fset.Position(r.info.Includes[1].Pos).String(); got != "x.gp:7:2" {
		t.Errorf("the position of ./physics.h is %s, want x.gp:7:2", got)
	}
	if got := r.fset.Position(r.info.Includes[0].Pos).String(); got != "x.gp:6:5" {
		t.Errorf("the position of <raylib.h> is %s, want x.gp:6:5 (the path)", got)
	}
	fs := r.info.NamespaceRenames["fs"]
	if fs == nil || fs.Spec != specs[3] || r.fset.Position(fs.Pos).String() != "x.gp:8:5" {
		t.Errorf("rename fs: %+v", fs)
	}
	if len(r.info.Packages) != 1 || r.info.Packages[0] != specs[0] {
		t.Errorf("Packages: %v", infoLines(r.info))
	}

	// The errors have the end of their node: the path, or the name.
	if r.errs != "x.gp:11:2: cannot dot-import a header\n" {
		t.Fatalf("errors: %s", r.errs)
	}
	bad := classifySource(t, importFile("\nvar rl = 1\n", `"<raylib.h"`, `rl "<raylib.h>"`, `rl "<x.h>"`, `"<y.h>"`, `"<y.h>"`))
	var ends []string
	for _, d := range bad.list.Diagnostics() {
		ends = append(ends, fmt.Sprintf("%d:%d-%d:%d", d.Pos.Line, d.Pos.Column, d.End.Line, d.End.Column))
	}
	wantEnds := []string{"4:2-4:13", "6:2-6:4", "8:2-8:9", "11:5-11:7"}
	if !slices.Equal(ends, wantEnds) {
		t.Errorf("error ranges %v, want %v\n%s", ends, wantEnds, bad.errs)
	}
}

func TestPackagePaths(t *testing.T) {
	const src = `package p

import (
	"fmt"
	"fmt"
	f "os"
	_ "strings"
	. "bytes"
	"C"
	C "math"
	""
	"a b"
	"<raylib.h>"
	"./physics.h"
	"./physics"
	"x.c"
	"::std"
	"myapp/api/v1"
)

import "errors"
`
	f, err := parser.ParseFile(token.NewFileSet(), "x.gp", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"fmt", "os", "strings", "bytes", "math", "x.c", "myapp/api/v1", "errors"}
	if got := PackagePaths(f); !slices.Equal(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestPackageLevelNames(t *testing.T) {
	const x = `package p

import "fmt"

func f() {}
func (T) m() {}
func init() {}
func _() {}

var a, b = 1, 2
var _ = 3

const (
	c = iota
	d
)

type T int
type G[P any] []P

func main() {
	local := 1
	_ = local
	fmt.Println()
}
`
	const y = `package p

var f = 2

func g() {}
`
	fset := token.NewFileSet()
	var files []*ast.File
	for _, src := range []struct{ name, src string }{{"x.gp", x}, {"y.gp", y}} {
		f, err := parser.ParseFile(fset, src.name, src.src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	got := make(map[string]string)
	for name, pos := range PackageLevelNames(files) {
		got[name] = fset.Position(pos).String()
	}
	want := map[string]string{
		"f":    "x.gp:5:6", // the first declaration
		"a":    "x.gp:10:5",
		"b":    "x.gp:10:8",
		"c":    "x.gp:14:2",
		"d":    "x.gp:15:2",
		"T":    "x.gp:18:6",
		"G":    "x.gp:19:6",
		"main": "x.gp:21:6",
		"g":    "y.gp:5:6",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestAssumedName(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"fmt", "fmt"},
		{"myapp/engine", "engine"},
		{"math/rand/v2", "rand"},
		{"v2", "v2"},
		{"gopkg.in/yaml.v3", "yaml"},
		{"github.com/mattn/go-sqlite3", "sqlite3"},
		{"example.com/foo-bar", "foo"},
		{"example.com/x/", "x"},
		{".", ""},
	} {
		if got := assumedName(tc.path); got != tc.want {
			t.Errorf("assumedName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestStrings(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{SystemHeader.String(), "SystemHeader"},
		{LocalHeader.String(), "LocalHeader"},
		{HeaderKind(0).String(), "HeaderKind(0)"},
		{HasName.String(), "HasName"},
		{DotImport.String(), "DotImport"},
		{UsedByPackage.String(), "UsedByPackage"},
		{NoNameReason(99).String(), "NoNameReason(99)"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}
