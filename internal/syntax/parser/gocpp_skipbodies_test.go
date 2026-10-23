// The tests of gocpp's patch 8 (../FORK.md §4): SkipFuncBodies leaves the bodies of functions out
// of the tree, but not those of init, macros and generic functions. This file is gocpp's own, not
// upstream's: ../patches/0008-skip-func-bodies.patch adds it.

package parser

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

const skipBodiesSource = `package p

import "fmt"

var f = func() int { return 1 }

type T struct{ a, b int }

func (t T) Method() int {
	{
		return t.a + len("}{") + int('}')
	}
}

func Plain(x int) (int, error) {
	// a comment with a } in it
	s := ` + "`raw }`" + `
	fmt.Println(s, struct{ n int }{1})
	return x, nil
}

func init() {
	println("init")
}

func Last() {}

func Gen[T any](x T) T {
	return x
}

type Box[T any] struct{ v T }

func (b Box[T]) Get() T { return b.v }

var after = 1
`

func parseSkipBodies(t *testing.T, mode Mode) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "skip.gp", skipBodiesSource, ParseComments|Phase10Syntax|mode)
	if err != nil {
		t.Fatal(err)
	}
	return fset, f
}

// funcBodies returns whether each function of the file has a body, by name.
func funcBodies(f *ast.File) map[string]bool {
	got := make(map[string]bool)
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			got[fn.Name.Name] = fn.Body != nil
		}
	}
	return got
}

func TestSkipFuncBodies(t *testing.T) {
	_, skipped := parseSkipBodies(t, SkipFuncBodies)
	got := funcBodies(skipped)

	t.Run("a function has no body", func(t *testing.T) {
		if got["Plain"] || got["Last"] {
			t.Errorf("bodies: %v", got)
		}
	})
	t.Run("a method has no body", func(t *testing.T) {
		if got["Method"] {
			t.Errorf("bodies: %v", got)
		}
	})
	t.Run("init keeps its body", func(t *testing.T) {
		if !got["init"] {
			t.Errorf("bodies: %v", got)
		}
	})
	// go/types reports a generic function without a body ("generic function is missing function
	// body"); a method of a generic type has no type parameters of its own, and loses its body.
	t.Run("a generic function keeps its body", func(t *testing.T) {
		if !got["Gen"] || got["Get"] {
			t.Errorf("bodies: %v", got)
		}
	})
	t.Run("a function literal outside a function keeps its body", func(t *testing.T) {
		var lits int
		ast.Inspect(skipped, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				lits++
			}
			return true
		})
		if lits != 1 {
			t.Errorf("%d function literals, want 1", lits)
		}
	})
	t.Run("the signatures are there", func(t *testing.T) {
		for _, decl := range skipped.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "Plain" {
				params := fn.Type.Params.NumFields()
				results := fn.Type.Results.NumFields()
				if params != 1 || results != 2 {
					t.Errorf("Plain's signature: %d parameters, %d results", params, results)
				}
			}
		}
	})
}

func TestSkipFuncBodiesKeepsPositions(t *testing.T) {
	fullSet, full := parseSkipBodies(t, 0)
	skipSet, skipped := parseSkipBodies(t, SkipFuncBodies)
	if len(full.Decls) != len(skipped.Decls) {
		t.Fatalf("%d declarations, want %d", len(skipped.Decls), len(full.Decls))
	}
	for i := range full.Decls {
		want, got := full.Decls[i], skipped.Decls[i]
		wantStart := fullSet.Position(want.Pos())
		gotStart := skipSet.Position(got.Pos())
		if wantStart != gotStart {
			t.Errorf("declaration %d starts at %v, want %v", i, gotStart, wantStart)
		}
		// A function without its body ends at its signature, the others where they did.
		if _, ok := want.(*ast.FuncDecl); ok {
			continue
		}
		wantEnd := fullSet.Position(want.End())
		gotEnd := skipSet.Position(got.End())
		if wantEnd != gotEnd {
			t.Errorf("declaration %d ends at %v, want %v", i, gotEnd, wantEnd)
		}
	}
}

func TestSkipFuncBodiesOffByDefault(t *testing.T) {
	_, full := parseSkipBodies(t, 0)
	for name, has := range funcBodies(full) {
		if !has {
			t.Errorf("%s has no body without SkipFuncBodies", name)
		}
	}
}

func TestSkipFuncBodiesUnterminated(t *testing.T) {
	src := "package p\n\nfunc F() {\n\tif true {\n}\n"
	_, err := ParseFile(token.NewFileSet(), "skip.gp", src, SkipFuncBodies)
	if err == nil || !strings.Contains(err.Error(), "expected '}'") {
		t.Errorf("err = %v, want an error that a } is missing", err)
	}
}

func TestSkipFuncBodiesKeepsMacros(t *testing.T) {
	src := "package p\n\nmacro Trace() {}\n\nfunc F() {}\n"
	f, err := ParseFile(token.NewFileSet(), "skip.gp", src, Phase10Syntax|SkipFuncBodies)
	if err != nil {
		t.Fatal(err)
	}
	got := funcBodies(f)
	if !got["#Trace"] || got["F"] {
		t.Errorf("bodies: %v", got)
	}
}
