package check

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// SourceText prints an expression as the user wrote it (the lowering and the interop checker make
// their messages with it), without touching the tree, and the holders and the declared types read
// the same package.
func TestSourceText(t *testing.T) {
	src := `package main

import (
	"strings"
	"fmt"

	cstdlib "<stdlib.h>"
	"<chrono>"
	"<vector>"
)

var count *C.wchar_t

func main() {
	s := "x" + fmt.Sprint(1)
	h := C.cstr(s)
	g := h
	var v std.vector[int]
	n := cstdlib.getenv("X")
	C.free(C.CString(s))
	fmt.Println(cstdlib.abs(-3), std.max(1, 2), v, n, g, &count, (cstdlib.abs(4)), std.chrono.seconds(3))
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	p := mainPkg(t, prog, r)
	var calls []*ast.CallExpr
	ast.Inspect(p.Load.Files[0].AST, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			calls = append(calls, c)
		}
		return true
	})
	got := map[string]bool{}
	for _, c := range calls {
		before := types.ExprString(c)
		got[p.SourceText(c)] = true
		if after := types.ExprString(c); after != before {
			t.Errorf("SourceText changed the tree: %s became %s", before, after)
		}
	}
	for _, want := range []string{
		`cstdlib.getenv("X")`, `cstdlib.abs(-3)`, `std.max(1, 2)`, `C.cstr(s)`,
		`C.free(C.CString(s))`, `C.CString(s)`, `cstdlib.abs(4)`, `std.chrono.seconds(3)`,
	} {
		if !got[want] {
			t.Errorf("no call is printed as %s; got %v", want, got)
		}
	}
	for text := range got {
		for _, bad := range []string{"C.std", "C.cstdlib", "gp_cgoh"} {
			if strings.Contains(text, bad) {
				t.Errorf("%q still has the rewrite's %s", text, bad)
			}
		}
	}

	// The holders: h and its copy g, not s.
	holders := map[string]bool{}
	for _, name := range []string{"s", "h", "g", "n"} {
		holders[name] = p.Holders().IsHolder(identNamed(t, p, name))
	}
	if !holders["h"] || !holders["g"] || holders["s"] || holders["n"] {
		t.Errorf("holders = %v, want h and g only", holders)
	}

	// The declared type, as the program wrote it.
	te := p.DeclaredTypeExpr(identNamed(t, p, "count"))
	if te == nil || types.ExprString(te) != "*C.wchar_t" || !p.IsCTypeExpr(te) {
		t.Errorf("DeclaredTypeExpr(count) = %v, want a C type *C.wchar_t", te)
	}
	if te := p.DeclaredTypeExpr(identNamed(t, p, "s")); te != nil {
		t.Errorf("s has no declared type, got %s", types.ExprString(te))
	}
}

// identNamed returns a use of the variable called name in main.
func identNamed(t *testing.T, p *Package, name string) ast.Expr {
	t.Helper()
	var found ast.Expr
	ast.Inspect(p.Load.Files[0].AST, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name && p.Info.Uses[id] != nil && found == nil {
			found = id
		}
		return true
	})
	if found == nil {
		t.Fatalf("no use of %s", name)
	}
	return found
}
