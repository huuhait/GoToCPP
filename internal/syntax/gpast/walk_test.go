package gpast_test

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// every is a program with every form of the Phase 10 syntax.
const every = `package util

import "fmt"

// Assert panics.
macro Assert(cond bool, msg lazy string, a, b lazy []int) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s", #file(), #line(), msg()))
	}
}

macro Dbg[T any](v T) T { return v }

macro VM() { #body() }

// Load loads.
#util.Trace()
#Recover[int](1)
func loadConfig!(path string) Config {
	#util.Assert(path != "", "path is empty")
	data := os.ReadFile!(path)
	x := parse![int](#util.Dbg(data))
	return x
}

#Locked(&s.mu)
func (s *Store) Put!(k string) {}

#Derive(String)
type Color int

func main!() {
	defer f.Close!()
}
`

func parse(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", src, parser.Phase10Syntax|parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	return fset, f
}

// TestPositionsOfEveryNode calls Pos and End on every node of a program that
// has every form: a node that panics (a ValueSpec without a name does) or
// has an end before its start breaks every tool that walks the tree.
func TestPositionsOfEveryNode(t *testing.T) {
	fset, f := parse(t, every)
	size := fset.File(f.Pos()).Size()
	n := 0
	ast.Inspect(f, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		n++
		pos, end := node.Pos(), node.End()
		if !pos.IsValid() || !end.IsValid() || end < pos {
			t.Errorf("%T: pos %d, end %d", node, pos, end)
		}
		if _, ok := node.(*ast.File); !ok && fset.Position(end).Offset > size {
			t.Errorf("%T at %s ends after the file", node, fset.Position(pos))
		}
		return true
	})
	if n < 100 {
		t.Errorf("only %d nodes", n)
	}
}

// TestDeclNameEnd checks the end of the names of declarations: a macro name
// is "#Name" in the tree, but Name in the source.
func TestDeclNameEnd(t *testing.T) {
	fset, f := parse(t, every)
	var names []string
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start, end := fset.Position(fd.Name.Pos()).Offset, fset.Position(gpast.DeclNameEnd(fd)).Offset
		names = append(names, every[start:end])
	}
	if got, want := strings.Join(names, " "), "Assert Dbg VM loadConfig! Put! main!"; got != want {
		t.Errorf("names in the source: %q, want %q", got, want)
	}
	// A macro at the end of the file, without a body: the tree is partial,
	// and the end of the name is in the file.
	const src = "package p\nmacro M"
	fset = token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.gp", src, parser.Phase10Syntax)
	if err == nil {
		t.Fatal("no error for a macro without parameters")
	}
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if got := fset.Position(gpast.DeclNameEnd(fd)).Offset; got != len(src) {
				t.Errorf("the end of the name is at offset %d, want %d", got, len(src))
			}
		}
	}
}

// TestLeftovers checks that Leftovers finds every form once, in source order,
// and nothing in Go.
func TestLeftovers(t *testing.T) {
	_, f := parse(t, every)
	counts := map[string]int{}
	for _, n := range gpast.Leftovers(f) {
		switch n := n.(type) {
		case *ast.Ident:
			if gpast.IsMust(n.Name) {
				counts["must"]++
			} else {
				counts["macro use"]++
			}
		case *ast.UnaryExpr:
			counts["lazy"]++
		case *ast.GenDecl:
			counts["on top"]++
		case *ast.FuncDecl:
			counts["macro decl"]++
		}
	}
	// must: loadConfig!, ReadFile!, parse!, Put!, main!, Close!
	// macro uses (not on top, not declarations): #file, #line, #body,
	// #util (Assert), #util (Dbg)
	// lazy: the type of msg, and the type of the field a, b
	want := map[string]int{"must": 6, "macro use": 5, "lazy": 2, "on top": 4, "macro decl": 3}
	for k, v := range want {
		if counts[k] != v {
			t.Errorf("%s: %d, want %d (all: %v)", k, counts[k], v, counts)
		}
	}
	for i, n := range gpast.Leftovers(f) {
		if i > 0 && n.Pos() < gpast.Leftovers(f)[i-1].Pos() {
			t.Errorf("leftover %d is before the one before it", i)
		}
	}

	const plain = "package p\n\nfunc f() { x := a != b; if !x { return } }\n\nfunc g(macro, lazy int) { _ = macro + lazy }\n"
	_, g := parse(t, plain)
	if got := gpast.Leftovers(g); len(got) != 0 {
		t.Errorf("Go code has leftovers: %v", got)
	}
}
