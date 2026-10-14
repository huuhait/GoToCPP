package load

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
	fork "github.com/huuhait/gocpp/internal/syntax/parser"
)

// The tests parse with the parser fork: it parses std.map, and RewriteFile
// needs its object resolution. The rules in the comments are those of the
// ticket of job 1.8a (Phase 01 README §2.5).

// rewriteResult is what rewritePackage returns.
type rewriteResult struct {
	fset     *token.FileSet
	files    []*ast.File // the files, after the rewrite
	infos    []*FileInfo
	rws      []*Rewrite
	list     *diag.List // the errors of ClassifyImports and RewriteFile
	errs     string     // list, as diag.Printer prints it
	typeErrs string     // the errors of go/types on the rewritten files
}

// rewritePackage parses the sources as the files x.gp, y.gp, ... of one
// package with the parser fork, classifies their imports with the
// package-level names of all of them, rewrites each file (with the names
// that its dot imports of stubPackages bring in, as the loader will pass
// them) and checks the invariants of each result (checkRewrite). Then it
// type-checks the package with go/types and FakeImportC. It fails on a
// parse error.
func rewritePackage(t *testing.T, srcs ...string) rewriteResult {
	t.Helper()
	return rewrite(t, false, srcs...)
}

// rewrite is rewritePackage. With broken, a parse error is no failure, and
// the package is not type-checked (gocpp doesn't type-check a package with
// syntax errors).
func rewrite(t *testing.T, broken bool, srcs ...string) rewriteResult {
	t.Helper()
	r := rewriteResult{fset: token.NewFileSet(), list: new(diag.List)}
	for i, src := range srcs {
		name := string(rune('x'+i)) + ".gp"
		f, err := fork.ParseFile(r.fset, name, src, fork.AllErrors)
		if err != nil && !broken {
			t.Fatalf("parse %s: %v\n%s", name, err, src)
		}
		r.files = append(r.files, f)
	}
	pkgLevel := PackageLevelNames(r.files)
	for _, f := range r.files {
		info := ClassifyImports(r.fset, f, pkgLevel, nil, r.list)
		before := snapshot(f)
		rw := RewriteFile(r.fset, f, info, pkgLevel, dotNames(t, info), r.list)
		checkRewrite(t, r.fset, f, info, rw, before, !broken)
		r.infos = append(r.infos, info)
		r.rws = append(r.rws, rw)
	}
	r.errs = printDiags(t, r.list)
	if !broken {
		r.typeErrs = typeCheck(t, r.fset, r.files)
	}
	return r
}

// typeCheck type-checks the files of one package with stock go/types,
// FakeImportC and stubImporter, as step 1.10 will do, and returns the
// errors as diag.Printer prints them.
func typeCheck(t *testing.T, fset *token.FileSet, files []*ast.File) string {
	t.Helper()
	var errs diag.List
	conf := types.Config{GoVersion: "go1.25", FakeImportC: true, Importer: stubImporter{t}, Error: errs.Collect}
	_, _ = conf.Check(files[0].Name.Name, fset, files, nil)
	return printDiags(t, &errs)
}

// dotNames returns the exported names of the packages that the file of
// info imports with a dot.
func dotNames(t *testing.T, info *FileInfo) map[string]bool {
	t.Helper()
	names := make(map[string]bool)
	for _, spec := range info.Packages {
		p, _ := importPath(spec)
		if spec.Name == nil || spec.Name.Name != "." || stubPackages[p] == "" {
			continue
		}
		pkg, _ := stubImporter{t}.Import(p)
		for _, name := range pkg.Scope().Names() {
			if token.IsExported(name) {
				names[name] = true
			}
		}
	}
	return names
}

// stubPackages are the gocpp packages that the test files import, as small
// Go sources for go/types.
var stubPackages = map[string]string{
	"fmt": `package fmt

type Stringer interface{ String() string }

func Println(a ...any) (n int, err error) { return }
`,
	"time": `package time

type Duration int64

func (d Duration) String() string { return "" }

type Location struct{}

func (l *Location) String() string { return "" }

var Local *Location
`,
	"strings": `package strings

func ToUpper(s string) string { return s }
`,
	"slices": `package slices

func Contains[S ~[]E, E comparable](s S, v E) bool { return false }
`,
	"math": `package math

func Sqrt(x float64) float64 { return x }
`,
}

// stubImporter imports the stubPackages for go/types.
type stubImporter struct{ t *testing.T }

func (imp stubImporter) Import(path string) (*types.Package, error) {
	src, ok := stubPackages[path]
	if !ok {
		return nil, fmt.Errorf("no stub for %s", path)
	}
	fset := token.NewFileSet()
	f, err := fork.ParseFile(fset, path+".go", src, 0)
	if err != nil {
		imp.t.Fatalf("stub %s: %v", path, err)
	}
	pkg, err := (&types.Config{GoVersion: "go1.25"}).Check(path, fset, []*ast.File{f}, nil)
	if err != nil {
		imp.t.Fatalf("stub %s: %v", path, err)
	}
	return pkg, nil
}

// code returns the rewritten file f as Go source, without its package
// clause and its comments: each import declaration on one line, as
// import ("fmt"; "C"), then each other declaration as gofmt prints it.
func code(t *testing.T, fset *token.FileSet, f *ast.File) string {
	t.Helper()
	var parts []string
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			var specs []string
			for _, s := range gd.Specs {
				spec := s.(*ast.ImportSpec)
				if spec.Name != nil {
					specs = append(specs, spec.Name.Name+" "+spec.Path.Value)
				} else {
					specs = append(specs, spec.Path.Value)
				}
			}
			parts = append(parts, "import ("+strings.Join(specs, "; ")+")")
			continue
		}
		var b bytes.Buffer
		cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
		if err := cfg.Fprint(&b, fset, d); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, b.String())
	}
	return strings.Join(parts, "\n\n")
}

// namespaces describes rw.Namespaces: each name with the line:column of
// its uses, as in "std 7:2 9:7; stdd 12:2".
func namespaces(fset *token.FileSet, rw *Rewrite) string {
	var parts []string
	for _, ns := range rw.Namespaces {
		s := ns.Name
		for _, u := range ns.Uses {
			p := fset.Position(u.Ident.Pos())
			s += fmt.Sprintf(" %d:%d", p.Line, p.Column)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "; ")
}

// useLines describes rw.Uses, one line per use: the line:column and the
// name that the user wrote, the kind, where the use comes from (a header,
// or a namespace and its rename), what the name became, and "bare" for a
// use without a selector.
func useLines(fset *token.FileSet, rw *Rewrite) []string {
	var lines []string
	for _, u := range rw.Uses {
		p := fset.Position(u.Ident.Pos())
		from := u.Namespace
		if u.Include != nil {
			from = u.Include.Path
		}
		if u.Rename != nil {
			from += " (" + u.Rename.Name + ")"
		}
		line := fmt.Sprintf("%d:%d %s %v %s %s", p.Line, p.Column, u.Ident.Name, u.Kind, from, u.Path)
		if u.Bare {
			line += " bare"
		}
		lines = append(lines, line)
	}
	return lines
}

// fileSnapshot is what checkRewrite needs to know about a file from before
// the rewrite.
type fileSnapshot struct {
	imports     []*ast.ImportSpec // the import specs in the declarations
	importDecls []*ast.GenDecl    // the import declarations
	usesC       bool              // the user wrote C: it is in f.Unresolved
}

func snapshot(f *ast.File) fileSnapshot {
	s := fileSnapshot{imports: fileImports(f)}
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			s.importDecls = append(s.importDecls, gd)
		}
	}
	for _, id := range f.Unresolved {
		if id.Name == "C" {
			s.usesC = true
		}
	}
	return s
}

// checkRewrite checks what must hold after every RewriteFile call on f. With
// allPos, it checks that every node has a position, which holds for a file
// without syntax errors.
func checkRewrite(t *testing.T, fset *token.FileSet, f *ast.File, info *FileInfo, rw *Rewrite, before fileSnapshot, allPos bool) {
	t.Helper()
	name := fset.Position(f.Package).Filename

	// Rules 1 and 6: the imports are the gocpp packages, in source order,
	// and, when the file uses C, one import "C" at the end of the first
	// import declaration that is left, at its import keyword, or in a new
	// declaration at the package keyword (finding 2 of round 1). f.Imports
	// agrees, no import declaration is empty, and the declarations are in
	// source order.
	want := slices.DeleteFunc(slices.Clone(before.imports), func(s *ast.ImportSpec) bool {
		return !slices.Contains(info.Packages, s)
	})
	if usesC := before.usesC || len(rw.Uses) > 0; usesC != (rw.ImportC != nil) {
		t.Errorf("%s: the file uses C: %v, but ImportC is %v", name, usesC, rw.ImportC)
	}
	var first *ast.GenDecl
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			if first == nil {
				first = gd
			}
			if len(gd.Specs) == 0 {
				t.Errorf("%s: an empty import declaration is left", name)
			}
		}
	}
	if c := rw.ImportC; c != nil {
		left := slices.IndexFunc(before.importDecls, func(gd *ast.GenDecl) bool { return slices.Contains(f.Decls, ast.Decl(gd)) })
		switch {
		case first == nil || first.Specs[len(first.Specs)-1] != c:
			t.Errorf("%s: import \"C\" is not the last import of the first import declaration", name)
		case c.Path.Value != `"C"` || c.Name != nil:
			t.Errorf("%s: ImportC is %v %s", name, c.Name, c.Path.Value)
		case left >= 0 && first != before.importDecls[left], left < 0 && (first != f.Decls[0] || first.Pos() != f.Package):
			t.Errorf("%s: import \"C\" is not in the first import declaration that is left, or in a new one first", name)
		case c.Pos() != first.Pos() || c.End() > first.End() || !c.Pos().IsValid():
			t.Errorf("%s: import \"C\" is at %v to %v, its declaration at %v to %v", name, c.Pos(), c.End(), first.Pos(), first.End())
		}
		want = append(want, c)
	}
	for i := 1; i < len(f.Decls); i++ {
		if f.Decls[i-1].End() > f.Decls[i].Pos() {
			t.Errorf("%s: declaration %d ends at %s, after declaration %d starts", name, i-1, fset.Position(f.Decls[i-1].End()), i)
		}
	}
	got := fileImports(f)
	if !slices.Equal(got, want) {
		t.Errorf("%s: the imports are %v, want %v", name, specPaths(got), specPaths(want))
	}
	if !slices.Equal(f.Imports, got) {
		t.Errorf("%s: f.Imports is %v, the declarations have %v", name, specPaths(f.Imports), specPaths(got))
	}

	// Rules 2, 3, 5 and 7: each use says where it comes from, its selector
	// is in the AST with the new X, and each new identifier is at the name
	// that the user wrote (rule 9).
	inAST := make(map[ast.Node]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		inAST[n] = true
		return true
	})
	prev := token.NoPos
	for _, u := range rw.Uses {
		pos := u.Ident.Pos()
		where := fmt.Sprintf("%s: the use of %s at %s", name, u.Ident.Name, fset.Position(pos))
		if pos <= prev {
			t.Errorf("%s: the uses are not in source order", where)
		}
		prev = pos
		if !inAST[u.Sel] {
			t.Errorf("%s: Sel is not in the AST", where)
		}
		if s := types.ExprString(u.Sel.X); s != u.Path {
			t.Errorf("%s: Path is %s, but Sel.X is %s", where, u.Path, s)
		}
		ast.Inspect(u.Sel.X, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Pos() != pos {
				t.Errorf("%s: the identifier %s in Sel.X is at %s", where, id.Name, fset.Position(id.Pos()))
			}
			return true
		})
		if u.Bare && (u.Kind == UnnamedHeaderUse || u.Sel.Sel != u.Ident || u.Sel.Pos() != pos || u.Sel.End() != u.Ident.End()) {
			t.Errorf("%s: a bare use with %+v, at %v to %v", where, u, u.Sel.Pos(), u.Sel.End())
		}
		switch u.Kind {
		case HeaderUse:
			if u.Include == nil || info.HeaderNames[u.Ident.Name] != u.Include || u.Path != "C" || u.Namespace != "" || u.Rename != nil {
				t.Errorf("%s: a HeaderUse with %+v", where, u)
			}
		case UnnamedHeaderUse:
			if u.Include == nil || u.Include.Name != "" || u.Include.WouldBe != u.Ident.Name || u.Include.Why == Blank ||
				u.Path != "C" || u.Namespace != "" || u.Rename != nil {
				t.Errorf("%s: an UnnamedHeaderUse with %+v", where, u)
			}
		case NamespaceUse:
			switch {
			case u.Include != nil:
				t.Errorf("%s: a NamespaceUse with an include", where)
			case u.Rename != nil:
				if info.NamespaceRenames[u.Ident.Name] != u.Rename || u.Namespace != u.Rename.Namespace ||
					!u.Bare && (u.Path != "C."+u.Ident.Name || u.Sel.X.(*ast.SelectorExpr).Sel != u.Ident) {
					t.Errorf("%s: a renamed NamespaceUse with %+v", where, u)
				}
			case u.Bare || u.Namespace != "::"+u.Ident.Name || u.Sel.X.(*ast.SelectorExpr).Sel != u.Ident:
				t.Errorf("%s: a NamespaceUse with %+v", where, u)
			}
		default:
			t.Errorf("%s: the kind is %v", where, u.Kind)
		}
		if inAST[u.Ident] != (u.Bare || u.Kind == NamespaceUse) {
			t.Errorf("%s: the name is in the AST: %v", where, inAST[u.Ident])
		}
	}

	// Rule 4: the candidates are the namespace uses that are not renames,
	// grouped by name in the order of their first use. Renames holds the
	// renames of the file, in source order.
	var cands []*CppUse
	seen := make(map[string]bool)
	for _, ns := range rw.Namespaces {
		if seen[ns.Name] || len(ns.Uses) == 0 {
			t.Errorf("%s: the candidate %s is there twice or has no uses", name, ns.Name)
		}
		seen[ns.Name] = true
		for _, u := range ns.Uses {
			if u.Ident.Name != ns.Name {
				t.Errorf("%s: the candidate %s has a use of %s", name, ns.Name, u.Ident.Name)
			}
		}
		cands = append(cands, ns.Uses...)
	}
	slices.SortFunc(cands, func(a, b *CppUse) int { return int(a.Ident.Pos() - b.Ident.Pos()) })
	wantCands := slices.DeleteFunc(slices.Clone(rw.Uses), func(u *CppUse) bool { return u.Kind != NamespaceUse || u.Rename != nil })
	if !slices.Equal(cands, wantCands) {
		t.Errorf("%s: the candidates have %d uses, want %d", name, len(cands), len(wantCands))
	}
	if len(rw.Renames) != len(info.NamespaceRenames) ||
		!slices.IsSortedFunc(rw.Renames, func(a, b *NamespaceRename) int { return int(a.Pos - b.Pos) }) {
		t.Errorf("%s: Renames has %d renames, not sorted or not the file's %d", name, len(rw.Renames), len(info.NamespaceRenames))
	}
	for _, rn := range rw.Renames {
		if info.NamespaceRenames[rn.Name] != rn {
			t.Errorf("%s: Renames has %s, which is not a rename of the file", name, rn.Name)
		}
	}

	// Rule 9: no node is without a position, each node lies inside its
	// parent (comments aside: a doc comment comes before its node), and in
	// a selector X ends before Sel starts, except that a new C shares the
	// first character of the name after it (C.std, C.fs, C.raylib).
	if allPos {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			switch n.(type) {
			case nil:
				stack = stack[:len(stack)-1]
				return false
			case *ast.CommentGroup, *ast.Comment:
				return false
			}
			if !n.Pos().IsValid() {
				t.Errorf("%s: a %T has no position", name, n)
			}
			if len(stack) > 0 {
				if p := stack[len(stack)-1]; n.Pos() < p.Pos() || n.End() > p.End() {
					t.Errorf("%s: a %T at %s to %s is outside its parent %T at %s to %s", name, n,
						fset.Position(n.Pos()), fset.Position(n.End()), p, fset.Position(p.Pos()), fset.Position(p.End()))
				}
			}
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.X.End() > sel.Sel.Pos() {
				if c, ok := sel.X.(*ast.Ident); !ok || c.Name != "C" || c.Pos() != sel.Sel.Pos() {
					t.Errorf("%s: in the selector %s at %s, X ends after Sel starts", name, types.ExprString(sel), fset.Position(sel.Pos()))
				}
			}
			stack = append(stack, n)
			return true
		})
	}
}

// specPaths returns the paths of the import specs, for messages.
func specPaths(specs []*ast.ImportSpec) []string {
	var paths []string
	for _, s := range specs {
		paths = append(paths, s.Path.Value)
	}
	return paths
}

type rewriteCase struct {
	name     string
	src      string   // x.gp
	more     []string // the other files of the package: y.gp, z.gp, ...
	want     string   // x.gp after the rewrite (see code)
	errs     string   // the errors of ClassifyImports and RewriteFile
	typeErrs string   // the errors of go/types on the rewritten package
	ns       string   // the namespace candidates of x.gp (see namespaces)
}

func runRewriteCases(t *testing.T, cases []rewriteCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := rewritePackage(t, append([]string{tc.src}, tc.more...)...)
			if got := code(t, r.fset, r.files[0]); got != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.want)
			}
			if r.errs != tc.errs {
				t.Errorf("errors:\n%s\nwant:\n%s", r.errs, tc.errs)
			}
			if r.typeErrs != tc.typeErrs {
				t.Errorf("go/types errors:\n%s\nwant:\n%s", r.typeErrs, tc.typeErrs)
			}
			if got := namespaces(r.fset, r.rws[0]); got != tc.ns {
				t.Errorf("namespaces: %q, want %q", got, tc.ns)
			}
		})
	}
}

// TestRewriteImports checks rules 1 and 6: the imports that go/types must
// not see go away, and import "C" comes in exactly when the file uses C.
func TestRewriteImports(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "README §2.5: headers go away, C comes into the first import declaration",
		src: `package p

import (
	"fmt"

	"<raylib.h>"
	"<vector>"
)

var v = std.vector[int]{1, 2}

var white = raylib.RAYWHITE

func main() {
	fmt.Println(v, white)
}
`,
		want: `import ("fmt"; "C")

var v = C.std.vector[int]{1, 2}

var white = C.RAYWHITE

func main() {
	fmt.Println(v, white)
}`,
		ns: "std 10:9",
	}, {
		name: "an import declaration that becomes empty goes away, and C gets a new one",
		src: `package p

import (
	"<raylib.h>"
)

func main() {
	raylib.InitWindow(800, 450, "demo")
}
`,
		want: `import ("C")

func main() {
	C.InitWindow(800, 450, "demo")
}`,
	}, {
		name: "several import declarations",
		src: `package p

import "<cstdio>"

import (
	"fmt"
	"<vector>"
	"strings"
)

import "<raylib.h>"

func main() {
	cstdio.printf("%s\n", C.cstr(strings.ToUpper("x")))
	fmt.Println()
}
`,
		want: `import ("fmt"; "strings"; "C")

func main() {
	C.printf("%s\n", C.cstr(strings.ToUpper("x")))
	fmt.Println()
}`,
	}, {
		name: "a file that doesn't use C gets no import C",
		src: `package p

import (
	"fmt"

	"<vector>"
	rl "<raylib.h>"
)

func main() {
	fmt.Println()
}
`,
		want: `import ("fmt")

func main() {
	fmt.Println()
}`,
	}, {
		name: "a file without imports that uses C",
		src: `package p

var size C.size_t = 16

func f(n C.int) C.int {
	return n
}
`,
		want: `import ("C")

var size C.size_t = 16

func f(n C.int) C.int {
	return n
}`,
	}, {
		name: "C and a header: one import C",
		src: `package p

import "<cstdio>"

func main() {
	cstdio.printf("%d\n", C.int(1))
}
`,
		want: `import ("C")

func main() {
	C.printf("%d\n", C.int(1))
}`,
	}, {
		name: "the import \"C\" of the user is an error and goes away; C still comes in once",
		src: `package p

import "C"

var n C.int
`,
		errs: `x.gp:3:8: import "C" is not allowed: C's built-in types need no import in gocpp, ` +
			`and there is no inline C preamble (put C code in a header next to the .gp file)` + "\n",
		want: `import ("C")

var n C.int`,
	}, {
		name: "imports with errors go away",
		src: `package p

import (
	"fmt"
	"<raylib.h"
	"::std"
	"./physics"
	C "<x.h>"
	string "<string.h>"
)

func main() {
	fmt.Println()
}
`,
		errs: `x.gp:5:2: invalid header path "<raylib.h": missing > at the end` + "\n" +
			`x.gp:6:2: namespace ::std needs no import: use std.X directly, or rename it: import cstd "::std"` + "\n" +
			`x.gp:7:2: invalid import path "./physics": a local header ends in .h, .hh, .hpp, .hxx, .h++ or .inl` + "\n" +
			"x.gp:8:2: cannot import header as C - C is reserved for C's built-in types\n" +
			"x.gp:9:2: cannot import header as string - string is a predeclared Go name (use another name, like cstring)\n",
		want: `import ("fmt")

func main() {
	fmt.Println()
}`,
	}, {
		name: "a gocpp package that is imported twice stays: go/types reports it",
		src: `package p

import (
	"fmt"
	"fmt"
	"<vector>"
)

func main() {
	fmt.Println(std.vector[int]{})
}
`,
		want: `import ("fmt"; "fmt"; "C")

func main() {
	fmt.Println(C.std.vector[int]{})
}`,
		typeErrs: "x.gp:5:2: fmt redeclared in this block\n\tx.gp:4:2: other declaration of fmt\n" +
			"x.gp:5:2: \"fmt\" imported and not used\n",
		ns: "std 10:14",
	}})
}

// TestRewriteHeaders checks rule 2: raylib.X becomes C.X when raylib is the
// package name of a header in this file.
func TestRewriteHeaders(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "default names, a chosen name and local headers",
		src: `package p

import (
	"<SDL2/SDL.h>"
	rl "<raylib.h>"
	"./physics.h"
	"./my-lib.h"
)

func main() {
	rl.InitWindow(800, 450, "demo")
	SDL.SDL_Init(SDL.SDL_INIT_VIDEO | SDL.SDL_INIT_AUDIO)
	physics.step(0.016)
	my_lib.f()
}
`,
		want: `import ("C")

func main() {
	C.InitWindow(800, 450, "demo")
	C.SDL_Init(C.SDL_INIT_VIDEO | C.SDL_INIT_AUDIO)
	C.step(0.016)
	C.f()
}`,
	}, {
		name: "a header imported twice under two names",
		src: `package p

import (
	"<raylib.h>"
	rl "<raylib.h>"
)

func main() {
	raylib.InitWindow(1, 2, "x")
	rl.CloseWindow()
}
`,
		want: `import ("C")

func main() {
	C.InitWindow(1, 2, "x")
	C.CloseWindow()
}`,
	}, {
		// The name stays in the AST of y.gp, where the rewrite doesn't
		// know it, and in x.gp, which imports a header, it is a namespace
		// (the probe will report "undefined: raylib").
		name: "a header package of another file",
		src: `package p

import "<vector>"

func f() {
	raylib.InitWindow(1, 2, "x")
}
`,
		more: []string{`package p

import "<raylib.h>"

func g() {
	raylib.CloseWindow()
}
`},
		want: `import ("C")

func f() {
	C.raylib.InitWindow(1, 2, "x")
}`,
		ns: "raylib 6:2",
	}, {
		name: "and the same in a file without headers",
		src: `package p

func f() {
	raylib.InitWindow(1, 2, "x")
}
`,
		more: []string{"package p\n\nimport \"<raylib.h>\"\n\nvar _ = raylib.RAYWHITE\n"},
		want: `func f() {
	raylib.InitWindow(1, 2, "x")
}`,
		typeErrs: "x.gp:4:2: undefined: raylib\n",
	}, {
		name: "a chosen name that is a package-level name of another file",
		src: `package p

import rl "<raylib.h>"

func f() int {
	return rl.x
}
`,
		more: []string{"package p\n\ntype T struct{ x int }\n\nvar rl T\n"},
		errs: "y.gp:5:5: rl already declared through import of <raylib.h>\n\tx.gp:3:8: other declaration of rl\n",
		want: `func f() int {
	return rl.x
}`,
	}, {
		// The parser doesn't resolve a non-name on the left of :=, so
		// RewriteFile can't know what it is. go/types reports Go's
		// error, after an "undefined" for the name.
		name: "a non-name on the left of := stays",
		src: `package p

import "<raylib.h>"

func f() {
	raylib.X := 1
}
`,
		want: `func f() {
	raylib.X := 1
}`,
		typeErrs: "x.gp:6:2: undefined: raylib\nx.gp:6:2: non-name raylib.X on left side of :=\n",
	}, {
		name: "the name of a header without a name is not a header use",
		src: `package p

import (
	"math"

	"<math.h>"
	_ "<GL/gl.h>"
)

func f() float64 {
	return math.Sqrt(2)
}
`,
		want: `import ("math")

func f() float64 {
	return math.Sqrt(2)
}`,
	}})
}

// TestRewriteNamespaces checks rules 3 and 4: in a file with a header, a
// name that Go declares nowhere is a namespace, and a rename expands to its
// path.
func TestRewriteNamespaces(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "namespaces, also nested ones: only the first name changes",
		src: `package p

import (
	"fmt"

	"<chrono>"
	"<map>"
	"<string>"
	"<vector>"
)

func main() {
	v := std.vector[int]{1, 2, 3}
	ages := std.map[std.string, int]{}
	d := std.chrono.seconds(5)
	now := std.chrono.steady_clock.now()
	it := std.vector[int].iterator{}
	fmt.Println(v, ages, d, now, it, glm.vec3(1, 2, 3))
}
`,
		want: `import ("fmt"; "C")

func main() {
	v := C.std.vector[int]{1, 2, 3}
	ages := C.std.map[C.std.string, int]{}
	d := C.std.chrono.seconds(5)
	now := C.std.chrono.steady_clock.now()
	it := C.std.vector[int].iterator{}
	fmt.Println(v, ages, d, now, it, C.glm.vec3(1, 2, 3))
}`,
		ns: "std 13:7 14:10 14:18 15:7 16:9 17:8; glm 18:35",
	}, {
		name: "only in a file that imports a header",
		src: `package p

import "fmt"

func main() {
	fmt.Println(std.vector[int]{})
}
`,
		want: `import ("fmt")

func main() {
	fmt.Println(std.vector[int]{})
}`,
		typeErrs: "x.gp:6:14: undefined: std\n",
	}, {
		name: "a header without a name is a header too",
		src: `package p

import _ "<GL/gl.h>"

var v = std.vector[int]{}
`,
		want: `import ("C")

var v = C.std.vector[int]{}`,
		ns: "std 5:9",
	}, {
		name: "typos are candidates too: the namespace probe reports them",
		src: `package p

import (
	"fmt"

	"<vector>"
)

func main() {
	w := stdd.vector[int]{}
	fmtt.Println(w, stdd.size(w))
}
`,
		want: `import ("fmt"; "C")

func main() {
	w := C.stdd.vector[int]{}
	C.fmtt.Println(w, C.stdd.size(w))
}`,
		// fmt is not used: the user meant it in fmtt.Println.
		typeErrs: "x.gp:4:2: \"fmt\" imported and not used\n",
		ns:       "stdd 10:7 11:18; fmtt 11:2",
	}, {
		name: "a rename becomes C.fs, with its path in the use",
		src: `package p

import (
	"fmt"

	fs "::std::filesystem"
	"<filesystem>"
)

func main() {
	p := fs.path("/tmp/x.txt")
	sep := fs.path.preferred_separator
	fmt.Println(p.filename(), sep, std.filesystem.exists(p))
}
`,
		want: `import ("fmt"; "C")

func main() {
	p := C.fs.path("/tmp/x.txt")
	sep := C.fs.path.preferred_separator
	fmt.Println(p.filename(), sep, C.std.filesystem.exists(p))
}`,
		ns: "std 13:33",
	}, {
		name: "renamed namespaces with one part and with keywords",
		src: `package p

import (
	cfmt "::fmt"
	x "::type::detail"
	"<fmt/core.h>"
)

var s = cfmt.format("{}", x.y)
`,
		want: `import ("C")

var s = C.cfmt.format("{}", C.x.y)`,
	}, {
		// The probe gets the rename (Rewrite.Renames), and can say that
		// no header declares the namespace.
		name: "a rename in a file without headers is still rewritten",
		src: `package p

import fs "::std::filesystem"

var p = fs.path("/tmp")
`,
		want: `import ("C")

var p = C.fs.path("/tmp")`,
	}, {
		name: "predeclared names, gocpp packages and C are not namespaces",
		src: `package p

import (
	"fmt"
	str "strings"

	"<vector>"
)

func main() {
	fmt.Println(str.ToUpper("x"), C.int(1), error.Error)
	_ = len.x
	_ = nil.x
	_ = string.npos
}
`,
		want: `import ("fmt"; str "strings"; "C")

func main() {
	fmt.Println(str.ToUpper("x"), C.int(1), error.Error)
	_ = len.x
	_ = nil.x
	_ = string.npos
}`,
		typeErrs: "x.gp:12:10: invalid use of len (built-in) in selector expression\n" +
			"x.gp:13:10: nil.x undefined (type untyped nil has no field or method x)\n" +
			"x.gp:14:13: string.npos undefined (type string has no field or method npos)\n",
	}, {
		name: "a name declared after its use in a block is a namespace there",
		src: `package p

import (
	"fmt"

	"<vector>"
)

func main() {
	v := std.vector[int]{}
	std := 1
	fmt.Println(v, std)
}
`,
		want: `import ("fmt"; "C")

func main() {
	v := C.std.vector[int]{}
	std := 1
	fmt.Println(v, std)
}`,
		ns: "std 10:7",
	}, {
		// Like a package name, a parenthesized namespace is no selector
		// root: (fmt).Println is an error in Go too.
		name: "a parenthesized name stays",
		src: `package p

import "<vector>"

var v = (std).vector
`,
		want:     `var v = (std).vector`,
		typeErrs: "x.gp:5:10: undefined: std\n",
	}})
}

// TestRewriteScopes checks rule 8: Go's scoping decides what a name is.
// After the rewrite, each case is valid Go (no go/types error), so each
// name that stays is really a Go name.
func TestRewriteScopes(t *testing.T) {
	const imports = `package p

import (
	"<raylib.h>"
	"<vector>"
)

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

`
	runRewriteCases(t, []rewriteCase{{
		name: "locals, parameters, results and receivers",
		src: imports + `func locals() int {
	std := T{}
	var raylib T
	return std.vector + raylib.InitWindow
}

func params(std T, raylib *T) int {
	return std.vector + raylib.InitWindow
}

func results() (std T, raylib T) {
	_ = std.vector + raylib.InitWindow
	return
}

func (std T) receiver() int {
	return std.vector
}

func (raylib *T) pointerReceiver() int {
	return raylib.InitWindow
}

func after() {
	_ = std.vector[int]{}
	raylib.CloseWindow()
}
`,
		want: `import ("C")

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func locals() int {
	std := T{}
	var raylib T
	return std.vector + raylib.InitWindow
}

func params(std T, raylib *T) int {
	return std.vector + raylib.InitWindow
}

func results() (std T, raylib T) {
	_ = std.vector + raylib.InitWindow
	return
}

func (std T) receiver() int {
	return std.vector
}

func (raylib *T) pointerReceiver() int {
	return raylib.InitWindow
}

func after() {
	_ = C.std.vector[int]{}
	C.CloseWindow()
}`,
		ns: "std 36:6",
	}, {
		// The parser leaves Ident.Obj nil for a type parameter of a
		// method's receiver (go.dev/issue/50956).
		name: "type parameters",
		src: imports + `type getter interface{ get() int }

func typeParam[std getter](x std) int {
	return std.get(x)
}

type Vec[raylib getter] struct{ x raylib }

func (v Vec[raylib]) receiverTypeParam() int {
	return raylib.get(v.x)
}

func (v *Vec[std]) pointerReceiverTypeParam() int {
	return std.get(v.x)
}
`,
		want: `type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

type getter interface{ get() int }

func typeParam[std getter](x std) int {
	return std.get(x)
}

type Vec[raylib getter] struct{ x raylib }

func (v Vec[raylib]) receiverTypeParam() int {
	return raylib.get(v.x)
}

func (v *Vec[std]) pointerReceiverTypeParam() int {
	return std.get(v.x)
}`,
	}, {
		name: "range, type switch, select, if and closure variables",
		src: imports + `func stmts(xs []T, m map[T]bool, v any, ch chan T) int {
	n := 0
	for _, std := range xs {
		n += std.vector
	}
	for raylib := range m {
		n += raylib.InitWindow
	}
	switch std := v.(type) {
	case T:
		n += std.vector
	}
	select {
	case raylib := <-ch:
		n += raylib.InitWindow
	}
	if std := (T{}); std.vector > 0 {
		n++
	}
	f := func(std T) int { return std.vector }
	return n + f(T{})
}
`,
		want: `type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func stmts(xs []T, m map[T]bool, v any, ch chan T) int {
	n := 0
	for _, std := range xs {
		n += std.vector
	}
	for raylib := range m {
		n += raylib.InitWindow
	}
	switch std := v.(type) {
	case T:
		n += std.vector
	}
	select {
	case raylib := <-ch:
		n += raylib.InitWindow
	}
	if std := (T{}); std.vector > 0 {
		n++
	}
	f := func(std T) int { return std.vector }
	return n + f(T{})
}`,
	}, {
		name: "local types and consts, and blocks",
		src: imports + `func decls() int {
	type std = T
	const raylib = 2
	{
		var std T
		_ = std.vector
	}
	return std.get(T{}) + raylib
}

func inner() {
	{
		std := T{}
		_ = std.vector
	}
	_ = std.vector[int]{}
}
`,
		want: `import ("C")

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func decls() int {
	type std = T
	const raylib = 2
	{
		var std T
		_ = std.vector
	}
	return std.get(T{}) + raylib
}

func inner() {
	{
		std := T{}
		_ = std.vector
	}
	_ = C.std.vector[int]{}
}`,
		ns: "std 27:6",
	}, {
		name: "package-level names of this file and of another file",
		src: imports + `var std = T{}

func f() int {
	return std.vector + vec.vector
}
`,
		more: []string{"package p\n\nvar vec T\n"},
		want: `type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

var std = T{}

func f() int {
	return std.vector + vec.vector
}`,
	}, {
		name: "a package-level name of another file hides a namespace in every file",
		src: imports + `func f() int {
	return std.vector
}
`,
		more: []string{"package p\n\nvar std T\n"},
		want: `type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func f() int {
	return std.vector
}`,
	}, {
		name: "method values, struct fields, composite literal keys and labels hide nothing",
		src: imports + `type S struct {
	std    int
	raylib T
}

func (S) vector() int { return 0 }

type U struct{}

func (U) std() int { return 0 }

func names() {
	s := S{std: 1, raylib: T{}}
	f := s.vector
	g := U{}.std
	_ = s.std + s.raylib.InitWindow + f() + g()
	m := map[int]int{std.npos: 1, raylib.KEY_A: 2}
std:
	for range m {
		break std
	}
	_ = std.vector[int]{}
}
`,
		want: `import ("C")

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

type S struct {
	std    int
	raylib T
}

func (S) vector() int { return 0 }

type U struct{}

func (U) std() int { return 0 }

func names() {
	s := S{std: 1, raylib: T{}}
	f := s.vector
	g := U{}.std
	_ = s.std + s.raylib.InitWindow + f() + g()
	m := map[int]int{C.std.npos: 1, C.KEY_A: 2}
std:
	for range m {
		break std
	}
	_ = C.std.vector[int]{}
}`,
		ns: "std 28:19 33:6",
	}, {
		// Finding 4 of round 1: Go declares a name after the expression
		// that gives it its value.
		name: "a name on the left of := or var is declared after its right side",
		src: imports + `func defines() {
	std := std.vector[int]{}
	var raylib = raylib.GetScreenWidth()
	_, _ = std, raylib
}

func stmts() {
	for std := range std.vector[int](3) {
		_ = std
	}
	if std := std.x; std != nil {
	}
	select {
	case std := <-std.ch():
		_ = std
	}
}
`,
		want: `import ("C")

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func defines() {
	std := C.std.vector[int]{}
	var raylib = C.GetScreenWidth()
	_, _ = std, raylib
}

func stmts() {
	for std := range C.std.vector[int](3) {
		_ = std
	}
	if std := C.std.x; std != nil {
	}
	select {
	case std := <-C.std.ch():
		_ = std
	}
}`,
		ns: "std 13:9 19:19 22:12 25:16",
	}, {
		name: "a parameter or a result typed by its own name, and function literals",
		src: imports + `func own(std std.vector[int]) (raylib raylib.Color) {
	_ = std
	return
}

var fl = func() (std std.string) {
	return
}

func deferred() (raylib T) {
	defer func() {
		raylib.InitWindow = 1
	}()
	return
}
`,
		want: `import ("C")

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func own(std C.std.vector[int]) (raylib C.Color) {
	_ = std
	return
}

var fl = func() (std C.std.string) {
	return
}

func deferred() (raylib T) {
	defer func() {
		raylib.InitWindow = 1
	}()
	return
}`,
		ns: "std 12:14 17:22",
	}, {
		name: "a method expression on a package-level type of another file",
		src: imports + `func f() int {
	return std.vector(std{})
}
`,
		more: []string{"package p\n\ntype std struct{}\n\nfunc (std) vector() int { return 0 }\n"},
		want: `type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func f() int {
	return std.vector(std{})
}`,
	}, {
		name: "types",
		src: `package p

import (
	"<SDL2/SDL.h>"
	"<string>"
	"<vector>"
)

type Window struct {
	w   *SDL.SDL_Window
	buf std.vector[int]
	SDL.SDL_Rect
}

type G[E any] struct{ x E }

var (
	v   std.vector[int]
	g   G[std.string]
	fn  func(std.string) *SDL.SDL_Window
	arr [4]std.string
	ch  chan *SDL.SDL_Event
	m   map[std.string][]std.vector[int]
)

func f(p *SDL.SDL_Window, xs ...std.string) (std.string, error) {
	var r SDL.SDL_Rect
	s := std.string("x")
	n := SDL.Uint32(5)
	w, ok := any(p).(*SDL.SDL_Window)
	_, _, _, _ = r, n, w, ok
	return s, nil
}
`,
		want: `import ("C")

type Window struct {
	w   *C.SDL_Window
	buf C.std.vector[int]
	C.SDL_Rect
}

type G[E any] struct{ x E }

var (
	v   C.std.vector[int]
	g   G[C.std.string]
	fn  func(C.std.string) *C.SDL_Window
	arr [4]C.std.string
	ch  chan *C.SDL_Event
	m   map[C.std.string][]C.std.vector[int]
)

func f(p *C.SDL_Window, xs ...C.std.string) (C.std.string, error) {
	var r C.SDL_Rect
	s := C.std.string("x")
	n := C.Uint32(5)
	w, ok := any(p).(*C.SDL_Window)
	_, _, _, _ = r, n, w, ok
	return s, nil
}`,
		ns: "std 11:6 18:6 19:8 20:11 21:9 23:10 23:23 26:33 26:46 28:7",
	}, {
		name: "composite literals",
		src: `package p

import (
	"<SDL2/SDL.h>"
	"<vector>"
)

var (
	r  = SDL.SDL_Rect{w: 640, h: 480}
	vs = []std.vector[int]{{1}, {2}}
	vv = std.vector[int]{1, 2}
	ps = map[string]*SDL.SDL_Point{"a": {x: 1}}
)
`,
		want: `import ("C")

var (
	r  = C.SDL_Rect{w: 640, h: 480}
	vs = []C.std.vector[int]{{1}, {2}}
	vv = C.std.vector[int]{1, 2}
	ps = map[string]*C.SDL_Point{"a": {x: 1}}
)`,
		ns: "std 10:9 11:7",
	}, {
		name: "a local hides only the name, not other uses",
		src: imports + `func f() int {
	std := T{}
	raylib.InitWindow(1, 2, "x")
	return std.vector
}
`,
		want: `import ("C")

type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func f() int {
	std := T{}
	C.InitWindow(1, 2, "x")
	return std.vector
}`,
	}, {
		name: "and a shadowed name is a Go error, not a C use",
		src: imports + `func f() {
	raylib := 1
	std := "x"
	raylib.InitWindow(1, 2, "x")
	_ = std.vector[int]{}
}
`,
		want: `type T struct{ vector, InitWindow int }

func (T) get() int { return 0 }

func f() {
	raylib := 1
	std := "x"
	raylib.InitWindow(1, 2, "x")
	_ = std.vector[int]{}
}`,
		typeErrs: "x.gp:15:9: raylib.InitWindow undefined (type int has no field or method InitWindow)\n" +
			"x.gp:16:10: std.vector is not a type\n",
	}})
}

// TestRewriteTypeSwitches checks finding 1 of round 1: the variable of a
// type switch is in scope in the clauses, not in the case lists, as in Go
// (the parser's resolver declares it over the case lists too).
func TestRewriteTypeSwitches(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "the variable is not in scope in the case lists",
		src: `package p

import (
	"fmt"

	"<raylib.h>"
	"<string>"
	"<vector>"
)

func f(v any) {
	switch std := v.(type) {
	case std.string, *std.vector[int]:
		fmt.Println(std)
	case raylib.Color:
		fmt.Println(std.size())
	}
	switch raylib := v.(type) {
	case raylib.Color, std.string:
		fmt.Println(raylib)
	}
	switch fmt := v.(type) {
	case fmt.Stringer:
		_ = fmt
	}
	switch v.(type) {
	case std.string:
	}
}
`,
		want: `import ("fmt"; "C")

func f(v any) {
	switch std := v.(type) {
	case C.std.string, *C.std.vector[int]:
		fmt.Println(std)
	case C.Color:
		fmt.Println(std.size())
	}
	switch raylib := v.(type) {
	case C.Color, C.std.string:
		fmt.Println(raylib)
	}
	switch fmt := v.(type) {
	case fmt.Stringer:
		_ = fmt
	}
	switch v.(type) {
	case C.std.string:
	}
}`,
		ns: "std 13:7 13:20 19:21 27:7",
	}, {
		name: "nested type switches",
		src: `package p

import (
	"<SDL2/SDL.h>"
	"<raylib.h>"
	"<string>"
)

func f(v, w any) {
	switch std := v.(type) {
	case std.string:
		switch raylib := w.(type) {
		case raylib.Color, *SDL.SDL_Window:
			_, _ = std, raylib
		}
	case raylib.Color:
		_ = std
	}
}
`,
		want: `import ("C")

func f(v, w any) {
	switch std := v.(type) {
	case C.std.string:
		switch raylib := w.(type) {
		case C.Color, *C.SDL_Window:
			_, _ = std, raylib
		}
	case C.Color:
		_ = std
	}
}`,
		ns: "std 11:7",
	}, {
		// In the clause, std is the variable of the outer switch, an int.
		name: "and the variable of an outer switch is in scope in an inner case list",
		src: `package p

import "<string>"

func f(v, w any) {
	switch std := v.(type) {
	case int:
		switch x := w.(type) {
		case std.string:
			_, _ = std, x
		}
	}
}
`,
		want: `func f(v, w any) {
	switch std := v.(type) {
	case int:
		switch x := w.(type) {
		case std.string:
			_, _ = std, x
		}
	}
}`,
		typeErrs: "x.gp:9:12: std.string is not a type\n",
	}})
}

// TestRewriteDotImports checks finding 3 of round 1: a name that a dot
// import brings in is a Go name, not a namespace (rewritePackage passes the
// names, as the loader will).
func TestRewriteDotImports(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "dot-imported names",
		src: `package p

import (
	. "time"

	"<vector>"
)

func f(d Duration) string {
	return Duration.String(d) + Local.String() + std.to_string(1)
}
`,
		want: `import (. "time"; "C")

func f(d Duration) string {
	return Duration.String(d) + Local.String() + C.std.to_string(1)
}`,
		ns: "std 10:47",
	}})
}

// TestRewriteBareUses checks finding 5 of round 1: a header package or a
// namespace rename without a selector gets go/types' error for a package,
// and go/types reports nothing more. A namespace stays: the probe decides.
func TestRewriteBareUses(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "bare and parenthesized names",
		src: `package p

import (
	"fmt"

	fs "::std::filesystem"
	"<raylib.h>"
	"<vector>"
)

type S struct{ raylib int }

func main() {
	x := raylib
	var y = fs
	(raylib).InitWindow(1, 2, "x")
	_ = (fs).path("/tmp")
	var w raylib
	s := S{raylib: 1}
	switch raylib := any(x).(type) {
	case raylib:
		_ = raylib
	}
	fmt.Println(x, y, w, s, std)
}
`,
		errs: "x.gp:14:7: use of package raylib not in selector\n" +
			"x.gp:15:10: use of package fs not in selector\n" +
			"x.gp:16:3: use of package raylib not in selector\n" +
			"x.gp:17:7: use of package fs not in selector\n" +
			"x.gp:18:8: use of package raylib not in selector\n" +
			"x.gp:21:7: use of package raylib not in selector\n",
		want: `import ("fmt"; "C")

type S struct{ raylib int }

func main() {
	x := C.raylib
	var y = C.fs
	(C.raylib).InitWindow(1, 2, "x")
	_ = (C.fs).path("/tmp")
	var w C.raylib
	s := S{raylib: 1}
	switch raylib := any(x).(type) {
	case C.raylib:
		_ = raylib
	}
	fmt.Println(x, y, w, s, std)
}`,
		typeErrs: "x.gp:24:26: undefined: std\n",
	}, {
		name: "a name that Go declares is no bare use",
		src: `package p

import "<raylib.h>"

func f(raylib int) int {
	return raylib
}
`,
		want: `func f(raylib int) int {
	return raylib
}`,
	}})
}

// TestRewriteConstantTypeArgs checks the type arguments of parser patch 2
// (step 1.3): only the names before the dots change.
func TestRewriteConstantTypeArgs(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "constant type arguments in types, composite literals and calls",
		src: `package p

import (
	"<array>"
	"<bitset>"
)

var (
	buf  std.array[byte, 256]
	bits = std.bitset[64]{}
	n    = std.array[int, 5]{}.size()
	m    = std.get[0](std.array[int, 2]{1, 2})
	nest std.array[std.array[int, 2], 3]
)
`,
		want: `import ("C")

var (
	buf  C.std.array[byte, 256]
	bits = C.std.bitset[64]{}
	n    = C.std.array[int, 5]{}.size()
	m    = C.std.get[0](C.std.array[int, 2]{1, 2})
	nest C.std.array[C.std.array[int, 2], 3]
)`,
		ns: "std 9:7 10:9 11:9 12:9 12:20 13:7 13:17",
	}})
}

// TestRewriteHint checks rule 5: a use of the name that a header without a
// name would have gets an error with a hint, and becomes C.X.
func TestRewriteHint(t *testing.T) {
	runRewriteCases(t, []rewriteCase{{
		name: "README §2.3: string.strlen",
		src: `package p

import (
	"fmt"

	"<string.h>"
)

func main() {
	fmt.Println(string.strlen("x"))
}
`,
		errs: `x.gp:10:14: <string.h> has no package name, because string is a predeclared Go name: ` +
			`give it a name: import cstring "<string.h>"` + "\n",
		want: `import ("fmt"; "C")

func main() {
	fmt.Println(C.strlen("x"))
}`,
	}, {
		name: "without the header, a predeclared name is a Go error",
		src: `package p

import "<vector>"

var n = string.strlen("x")
`,
		want:     `var n = string.strlen("x")`,
		typeErrs: "x.gp:5:16: string.strlen undefined (type string has no field or method strlen)\n",
	}, {
		name: "a name that Go declares gets no hint",
		src: `package p

import "<string.h>"

type T struct{ strlen int }

func f(string T) int {
	return string.strlen
}
`,
		want: `type T struct{ strlen int }

func f(string T) int {
	return string.strlen
}`,
	}, {
		name: "error.Error is Go: a method expression",
		src: `package p

import "<error.h>"

var f = error.Error

var g = error.error_at_line
`,
		errs: `x.gp:7:9: <error.h> has no package name, because error is a predeclared Go name: ` +
			`give it a name: import cerror "<error.h>"` + "\n",
		want: `import ("C")

var f = error.Error

var g = C.error_at_line`,
	}, {
		name: "a name chosen in the import that is an error",
		src: `package p

import string "<string.h>"

var n = string.strlen("x")
`,
		errs: "x.gp:3:8: cannot import header as string - string is a predeclared Go name (use another name, like cstring)\n" +
			`x.gp:5:9: <string.h> has no package name, because string is a predeclared Go name: ` +
			`give it another name: import cstring "<string.h>"` + "\n",
		want: `import ("C")

var n = C.strlen("x")`,
	}, {
		// Finding 7 of round 1: cstring is the name of <cstring>, and
		// cstring2 a package-level name.
		name: "the suggested name is free",
		src: `package p

import (
	"<string.h>"
	"<cstring>"
)

var cstring2 = 1

var n = string.strlen("x")
`,
		errs: `x.gp:10:9: <string.h> has no package name, because string is a predeclared Go name: ` +
			`give it a name: import cstring3 "<string.h>"` + "\n",
		want: `import ("C")

var cstring2 = 1

var n = C.strlen("x")`,
	}, {
		name: "two headers with the same name",
		src: `package p

import (
	"<time.h>"
	"<sys/time.h>"
	"./util/time.hpp"
)

var t = time.time(nil)
`,
		errs: `x.gp:9:9: <time.h> has no package name, because <sys/time.h> and "./util/time.hpp" would have ` +
			`the same name: give it a name: import ctime "<time.h>"` + "\n",
		want: `import ("C")

var t = C.time(nil)`,
	}, {
		name: "a dot import and init",
		src: `package p

import (
	. "<raylib.h>"
	"./init.h"
)

func main() {
	raylib.InitWindow(1, 2, "x")
	init.setup()
}
`,
		errs: "x.gp:4:2: cannot dot-import a header\n" +
			`x.gp:9:2: <raylib.h> has no package name, because it is dot-imported: give it a name: import raylib "<raylib.h>"` + "\n" +
			`x.gp:10:2: "./init.h" has no package name, because init can only be a func: give it a name: import cinit "./init.h"` + "\n",
		want: `import ("C")

func main() {
	C.InitWindow(1, 2, "x")
	C.setup()
}`,
	}, {
		name: "a header imported with _ gets no hint: the name can still be a namespace",
		src: `package p

import (
	_ "<GL/gl.h>"
	_ "<glm/glm.hpp>"
)

func main() {
	gl.glClear(0)
	_ = glm.vec3(1, 2, 3)
}
`,
		want: `import ("C")

func main() {
	C.gl.glClear(0)
	_ = C.glm.vec3(1, 2, 3)
}`,
		ns: "gl 9:2; glm 10:6",
	}, {
		name: "a name that a gocpp package or another file uses gets no hint",
		src: `package p

import (
	"math"

	"<math.h>"
	"<raylib.h>"
)

func f() float64 {
	return math.Sqrt(raylib.x)
}
`,
		more: []string{"package p\n\ntype T struct{ x float64 }\n\nvar raylib T\n"},
		want: `import ("math")

func f() float64 {
	return math.Sqrt(raylib.x)
}`,
	}})
}

// TestRewriteRecord checks rule 7: what each use records.
func TestRewriteRecord(t *testing.T) {
	const src = `package p

import (
	"fmt"

	fs "::std::filesystem"
	cfmt "::fmt"
	"<filesystem>"
	"<string.h>"
	rl "<raylib.h>"
	"./physics.h"
)

func main() {
	p := fs.path("/tmp")
	rl.InitWindow(1, 2, cfmt.format("{}", p.string()))
	fmt.Println(std.chrono.seconds(1), physics.g, string.strlen("x"), std.size(p))
}
`
	r := rewritePackage(t, src)
	rw := r.rws[0]
	want := []string{
		"15:7 fs NamespaceUse ::std::filesystem (fs) C.fs",
		"16:2 rl HeaderUse <raylib.h> C",
		"16:22 cfmt NamespaceUse ::fmt (cfmt) C.cfmt",
		"17:14 std NamespaceUse ::std C.std",
		"17:37 physics HeaderUse ./physics.h C",
		"17:48 string UnnamedHeaderUse <string.h> C",
		"17:68 std NamespaceUse ::std C.std",
	}
	if got := useLines(r.fset, rw); !slices.Equal(got, want) {
		t.Errorf("uses:\n\t%s\nwant:\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	if got := namespaces(r.fset, rw); got != "std 17:14 17:68" {
		t.Errorf("namespaces: %s", got)
	}
	var renames []string
	for _, rn := range rw.Renames {
		renames = append(renames, fmt.Sprintf("%s %s %s", rn.Name, rn.Namespace, r.fset.Position(rn.Pos)))
	}
	if want := []string{"fs ::std::filesystem x.gp:6:5", "cfmt ::fmt x.gp:7:7"}; !slices.Equal(renames, want) {
		t.Errorf("renames: %q, want %q", renames, want)
	}

	// Each use points into the AST: the selector, and the include or the
	// rename of the file.
	if u := rw.Uses[1]; u.Include != r.infos[0].HeaderNames["rl"] || types.ExprString(u.Sel) != "C.InitWindow" {
		t.Errorf("rl: %+v", u)
	}
	if u := rw.Uses[0]; u.Rename != r.infos[0].NamespaceRenames["fs"] || types.ExprString(u.Sel) != "C.fs.path" || u.Namespace != "::std::filesystem" {
		t.Errorf("fs: %+v", u)
	}
	if u := rw.Uses[3]; types.ExprString(u.Sel) != "C.std.chrono" || u.Sel.X.(*ast.SelectorExpr).Sel != u.Ident {
		t.Errorf("std: %+v, Sel %s", u, types.ExprString(u.Sel))
	}
	if r.errs == "" || r.typeErrs != "" {
		t.Errorf("errors: %q (want the hint), go/types: %q", r.errs, r.typeErrs)
	}
}

// TestRewritePositions checks rule 9: the new nodes are at what the user
// wrote, so errors are there too. checkRewrite checks every new identifier
// of every case; this test checks the errors and the import "C".
func TestRewritePositions(t *testing.T) {
	const src = `package p

import (
	"fmt"

	"<raylib.h>"
	"<string.h>"
	fs "::std::filesystem"
)

type S struct{ a int }

func main() {
	fmt.Println(C.int(1))
	_ = S{raylib.KEY: 1}
	_ = S{fs.x: 1}
	_ = string.strlen
}
`
	r := rewritePackage(t, src)

	// go/types reports the error at what the user wrote. The message
	// shows the rewritten name: the diagnostics can show raylib.KEY with
	// the Rewrite.
	want := "x.gp:15:8: invalid field name C.KEY in struct literal\n" +
		"x.gp:16:8: invalid field name C.fs.x in struct literal\n"
	if r.typeErrs != want {
		t.Errorf("go/types errors:\n%s\nwant:\n%s", r.typeErrs, want)
	}

	// The hint is at the name, from its start to its end.
	ds := r.list.Diagnostics()
	if len(ds) != 1 || ds[0].Pos.String() != "x.gp:17:6" || ds[0].End.String() != "x.gp:17:12" {
		t.Errorf("the hint is at %v, want x.gp:17:6 to x.gp:17:12", ds)
	}

	// The import "C" is at the import keyword of the first import
	// declaration (finding 2 of round 1).
	rw := r.rws[0]
	if got := r.fset.Position(rw.ImportC.Pos()).String(); got != "x.gp:3:1" {
		t.Errorf("import \"C\" is at %s, want x.gp:3:1", got)
	}

	// In a declaration without parentheses, it is at the end of the list,
	// so the declaration still ends at the end of its first import.
	r = rewritePackage(t, "package p\n\nimport \"fmt\"\nimport \"<raylib.h>\"\n\nfunc f() {\n\traylib.X()\n\tfmt.Println()\n}\n")
	gd := r.files[0].Decls[0].(*ast.GenDecl)
	if c := r.rws[0].ImportC; c.Pos() != gd.Pos() || len(gd.Specs) != 2 || gd.Specs[1] != c || r.fset.Position(gd.End()).String() != "x.gp:3:13" {
		t.Errorf("the declaration of import \"C\" is %s, at %s to %s", code(t, r.fset, r.files[0]), r.fset.Position(gd.Pos()), r.fset.Position(gd.End()))
	}

	// With no import declaration left, it is in a new one at the package
	// keyword.
	r = rewritePackage(t, "package p\n\nimport \"<vector>\"\n\nvar v = std.vector[int]{}\n\nvar n C.int\n")
	gd = r.files[0].Decls[0].(*ast.GenDecl)
	if got := r.fset.Position(gd.Pos()).String(); got != "x.gp:1:1" || r.rws[0].ImportC.Pos() != gd.Pos() {
		t.Errorf("the import declaration of C is at %s, want x.gp:1:1", got)
	}
}

// TestRewriteSelectorPositions checks the positions in selectors that go
// over two lines or have a comment (finding 4 of round 1).
func TestRewriteSelectorPositions(t *testing.T) {
	const src = `package p

import "<raylib.h>"

type S struct{ a int }

var (
	v = std.
		vector[int]{}
	w = raylib /* a header */ .RAYWHITE
	u = raylib. /* a header */ RAYWHITE
	s = S{raylib.
		KEY: 1}
)
`
	r := rewritePackage(t, src)
	want := []string{
		"8:6 std NamespaceUse ::std C.std",
		"10:6 raylib HeaderUse <raylib.h> C",
		"11:6 raylib HeaderUse <raylib.h> C",
		"12:8 raylib HeaderUse <raylib.h> C",
	}
	if got := useLines(r.fset, r.rws[0]); !slices.Equal(got, want) {
		t.Errorf("uses:\n\t%s\nwant:\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
	// The selector starts at the name and ends at the name after the dot.
	var ranges []string
	for _, u := range r.rws[0].Uses {
		ranges = append(ranges, fmt.Sprintf("%s-%s", r.fset.Position(u.Sel.Pos()), r.fset.Position(u.Sel.End())))
	}
	wantRanges := []string{"x.gp:8:6-x.gp:9:9", "x.gp:10:6-x.gp:10:37", "x.gp:11:6-x.gp:11:37", "x.gp:12:8-x.gp:13:6"}
	if !slices.Equal(ranges, wantRanges) {
		t.Errorf("the selectors are at %q, want %q", ranges, wantRanges)
	}
	if want := "x.gp:12:8: invalid field name C.KEY in struct literal\n"; r.typeErrs != want {
		t.Errorf("go/types errors:\n%s\nwant:\n%s", r.typeErrs, want)
	}
}

// TestRewriteInteropFiles is the end-to-end check: the interop files of the
// spike (spikes/2026-10-13/frontend, copied to testdata/rewrite) type-check
// without errors after the rewrite. The copy of case_P04_std_containers.gp
// has std.map, which the parser fork reads; the spike had to write std.MAP
// for the stock parser.
func TestRewriteInteropFiles(t *testing.T) {
	for _, tc := range []struct {
		file string
		ns   string // the namespace candidates
		uses int    // the number of rewritten uses
	}{
		{"case_P04_raylib.gp", "", 10},
		{"case_P04_sqlite.gp", "", 5},
		{"case_P04_std_containers.gp", "std 12:10 12:18 17:12", 3},
		{"case_root_README_example.gp", "std 18:9 31:10", 3},
		{"ok.gp", "std 18:38 25:7 30:10 36:9 41:7 59:20", 19},
	} {
		t.Run(tc.file, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", "rewrite", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			r := rewritePackage(t, string(src))
			if r.errs != "" || r.typeErrs != "" {
				t.Errorf("errors:\n%s\ngo/types errors:\n%s", r.errs, r.typeErrs)
			}
			if got := namespaces(r.fset, r.rws[0]); got != tc.ns {
				t.Errorf("namespaces: %q, want %q", got, tc.ns)
			}
			if got := len(r.rws[0].Uses); got != tc.uses {
				t.Errorf("%d uses, want %d:\n\t%s", got, tc.uses, strings.Join(useLines(r.fset, r.rws[0]), "\n\t"))
			}
		})
	}
}

// TestRewriteBrokenCode is the end-to-end check with mistakes: the edge
// cases of the spike (testdata/rewrite/edge.gp).
func TestRewriteBrokenCode(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "rewrite", "edge.gp"))
	if err != nil {
		t.Fatal(err)
	}
	r := rewritePackage(t, string(src))

	// E7: the hint for <string.h>.
	wantErrs := `x.gp:49:14: <string.h> has no package name, because string is a predeclared Go name: ` +
		`give it a name: import cstring "<string.h>"` + "\n"
	if r.errs != wantErrs {
		t.Errorf("errors:\n%s\nwant:\n%s", r.errs, wantErrs)
	}

	// E2: a promoted method of an embedded C++ type is a known gap
	// (Phase 12, gated by step 1.12); E5: mistakes in Go code. E1 and E4
	// give no error: the opaque pass (step 1.11) and the gates handle them.
	wantTypeErrs := "x.gp:26:5: mv.push_back undefined (type MyVec has no field or method push_back)\n" +
		"x.gp:40:16: cannot use \"nope\" (untyped string constant) as int value in variable declaration\n" +
		"x.gp:41:2: undefined: undefinedThing\n"
	if r.typeErrs != wantTypeErrs {
		t.Errorf("go/types errors:\n%s\nwant:\n%s", r.typeErrs, wantTypeErrs)
	}

	// E6: the typos stdd and fmtt are namespace candidates, which the
	// namespace probe reports as undefined.
	if got, want := namespaces(r.fset, r.rws[0]), "std 19:2 29:7; stdd 45:7; fmtt 46:2"; got != want {
		t.Errorf("namespaces: %q, want %q", got, want)
	}
}

// TestRewriteBrokenFiles rewrites files with syntax errors, as the language
// server does: RewriteFile must not panic, and the invariants hold.
func TestRewriteBrokenFiles(t *testing.T) {
	for _, src := range []string{
		"package p\n\nimport \"<vector>\"\n\nvar v = std.\n",
		"package p\n\nimport \"<raylib.h>\"\n\nfunc f( { raylib.x }\n",
		"package p\n\nimport (\n\t\"<vector>\"\n\nfunc f() { std.vector[int]{} }\n",
		"package p\n\nimport 42\nimport \"<vector>\"\n\nvar _ = std.x\n",
		"package p\n\nimport \"<vector>\"\n\nvar v = std.vector[\n",
		"package p\n\nimport \"<raylib.h>\"\n\nfunc f() { raylib. }\n",
		"package p\n\nimport \"<vector>\"\nvar x = 1\nimport \"<raylib.h>\"\n\nvar _ = raylib.x + std.y\n",
		"package p\n\nfunc f() { C. }\n",
		"package p",
		"",
	} {
		rewrite(t, true, src)
	}
}

// TestRewriteWithoutObjectResolution checks that RewriteFile refuses a file
// without the parser's object resolution: it would take every name for a
// declared one.
func TestRewriteWithoutObjectResolution(t *testing.T) {
	fset := token.NewFileSet()
	f, err := fork.ParseFile(fset, "x.gp", "package p\n\nimport \"<raylib.h>\"\n\nvar _ = raylib.x\n", fork.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var errs diag.List
	info := ClassifyImports(fset, f, nil, nil, &errs)
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "without object resolution") {
			t.Errorf("RewriteFile panicked with %q, want the object resolution message", msg)
		}
	}()
	RewriteFile(fset, f, info, nil, nil, &errs)
}

// TestReplaceChild checks the helper of the bare uses: it replaces a name in
// a field of type ast.Expr or []ast.Expr, and nowhere else.
func TestReplaceChild(t *testing.T) {
	id := &ast.Ident{Name: "raylib"}
	repl := &ast.SelectorExpr{X: &ast.Ident{Name: "C"}, Sel: id}
	paren := &ast.ParenExpr{X: id}
	call := &ast.CallExpr{Fun: &ast.Ident{Name: "f"}, Args: []ast.Expr{&ast.Ident{Name: "a"}, id}}
	field := &ast.Field{Names: []*ast.Ident{id}, Type: &ast.Ident{Name: "int"}}
	for _, tc := range []struct {
		parent ast.Node
		want   bool
	}{
		{paren, true},
		{call, true},
		{field, false}, // a name that is declared, not used
		{&ast.ParenExpr{X: &ast.Ident{Name: "raylib"}}, false},
		{&ast.BadExpr{}, false},
	} {
		if got := replaceChild(tc.parent, id, repl); got != tc.want {
			t.Errorf("replaceChild(%T) = %v, want %v", tc.parent, got, tc.want)
		}
	}
	if paren.X != repl || call.Args[1] != repl || call.Args[0] == repl || field.Names[0] != id {
		t.Errorf("the replacements are wrong: %v %v %v", paren.X, call.Args, field.Names)
	}
}

func TestCppUseKindString(t *testing.T) {
	for _, tc := range []struct {
		kind CppUseKind
		want string
	}{
		{HeaderUse, "HeaderUse"},
		{NamespaceUse, "NamespaceUse"},
		{UnnamedHeaderUse, "UnnamedHeaderUse"},
		{CppUseKind(0), "CppUseKind(0)"},
	} {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
