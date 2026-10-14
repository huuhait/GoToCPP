// gocpp: the tests of patch 1 (../FORK.md §4): a Go keyword right after a
// '.' on the same line is a name, for C++ names such as std::map and
// node.type(). This file is new in the patch.

package parser_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// kwKeywords are the 25 keywords of the Go spec. The scanner's tests check
// the list against go/token.
var kwKeywords = []string{
	"break", "case", "chan", "const", "continue",
	"default", "defer", "else", "fallthrough", "for",
	"func", "go", "goto", "if", "import",
	"interface", "map", "package", "range", "return",
	"select", "struct", "switch", "type", "var",
}

// kwParseFile parses src. A syntax error fails the test.
func kwParseFile(t *testing.T, src string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "test.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %q: %v", src, err)
	}
	return f
}

// kwParseBody parses body as the body of a function and returns its
// statements. A syntax error fails the test.
func kwParseBody(t *testing.T, body string) []ast.Stmt {
	t.Helper()
	f := kwParseFile(t, "package p\n\nfunc _() {\n"+body+"\n}\n")
	return f.Decls[0].(*ast.FuncDecl).Body.List
}

// kwFirstError parses src, which must have a syntax error, and returns the
// first error.
func kwFirstError(t *testing.T, src string) *scanner.Error {
	t.Helper()
	_, err := parser.ParseFile(token.NewFileSet(), "test.gp", src, 0)
	var list scanner.ErrorList
	if !errors.As(err, &list) || len(list) == 0 {
		t.Fatalf("parsing %q: got error %v, want a scanner.ErrorList", src, err)
	}
	return list[0]
}

// kwTree writes an expression with the types of its nodes, for example
// Call(Sel(node, type)) for node.type(). A missing expression is "-".
func kwTree(e ast.Expr) string {
	switch e := e.(type) {
	case nil:
		return "-"
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.SelectorExpr:
		return "Sel(" + kwTree(e.X) + ", " + e.Sel.Name + ")"
	case *ast.CallExpr:
		if e.Ellipsis.IsValid() {
			return "Call(" + kwTree(e.Fun) + kwList(e.Args) + "...)" // f(xs...)
		}
		return "Call(" + kwTree(e.Fun) + kwList(e.Args) + ")"
	case *ast.IndexExpr:
		return "Index(" + kwTree(e.X) + "; " + kwTree(e.Index) + ")"
	case *ast.IndexListExpr:
		return "IndexList(" + kwTree(e.X) + kwList(e.Indices) + ")"
	case *ast.CompositeLit:
		return "Lit(" + kwTree(e.Type) + kwList(e.Elts) + ")"
	case *ast.KeyValueExpr:
		return kwTree(e.Key) + ": " + kwTree(e.Value)
	case *ast.TypeAssertExpr:
		if e.Type == nil {
			return "Assert(" + kwTree(e.X) + "; type)" // x.(type)
		}
		return "Assert(" + kwTree(e.X) + "; " + kwTree(e.Type) + ")"
	case *ast.BinaryExpr:
		return "Binary(" + kwTree(e.X) + " " + e.Op.String() + " " + kwTree(e.Y) + ")"
	case *ast.UnaryExpr:
		return "Unary(" + e.Op.String() + kwTree(e.X) + ")"
	case *ast.StarExpr:
		return "Star(" + kwTree(e.X) + ")"
	case *ast.ArrayType:
		return "Array(" + kwTree(e.Len) + "; " + kwTree(e.Elt) + ")"
	case *ast.Ellipsis:
		return "Ellipsis(" + kwTree(e.Elt) + ")"
	case *ast.MapType:
		return "Map(" + kwTree(e.Key) + "; " + kwTree(e.Value) + ")"
	case *ast.ChanType:
		return "Chan(" + kwTree(e.Value) + ")"
	case *ast.FuncType:
		return "Func"
	case *ast.InterfaceType:
		return "Interface"
	case *ast.StructType:
		return "Struct"
	}
	return fmt.Sprintf("%T", e)
}

// kwList writes list for kwTree: "; a, b", or "" if list is empty.
func kwList(list []ast.Expr) string {
	var b strings.Builder
	for i, e := range list {
		if i == 0 {
			b.WriteString("; ")
		} else {
			b.WriteString(", ")
		}
		b.WriteString(kwTree(e))
	}
	return b.String()
}

// kwExpr returns the expression of a statement "x := e", "_ = e" or "e".
func kwExpr(t *testing.T, s ast.Stmt) ast.Expr {
	t.Helper()
	switch s := s.(type) {
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			return s.Rhs[0]
		}
	case *ast.ExprStmt:
		return s.X
	}
	t.Fatalf("got a %T, want x := e, _ = e or e", s)
	return nil
}

func TestKeywordSelectors(t *testing.T) {
	for _, test := range []struct{ stmt, want string }{
		// The examples of the plan (phase 01 README §2.2).
		{"m := std.map[std.string, int]{}", "Lit(IndexList(Sel(std, map); Sel(std, string), int))"},
		{"t := node.type()", "Call(Sel(node, type))"},
		{"n := posix.select(fd+1, &rd, nil, nil, nil)", "Call(Sel(posix, select); Binary(fd + 1), Unary(&rd), nil, nil, nil)"},

		// Space and comments on the same line don't matter.
		{"_ = x . map", "Sel(x, map)"},
		{"_ = x./* c */map", "Sel(x, map)"},

		// Chains, and keyword names inside other expressions.
		{"_ = x.map.range.type()", "Call(Sel(Sel(Sel(x, map), range), type))"},
		{"_ = std.map[int, int]{1: 2}[1]", "Index(Lit(IndexList(Sel(std, map); int, int); 1: 2); 1)"},
		{"x.default(x.case, x.go)", "Call(Sel(x, default); Sel(x, case), Sel(x, go))"},

		// Not a keyword: like upstream, the name may be on the next line.
		{"_ = x.\n\tfoo", "Sel(x, foo)"},
	} {
		stmts := kwParseBody(t, test.stmt)
		if len(stmts) != 1 {
			t.Errorf("parsing %q: got %d statements, want 1", test.stmt, len(stmts))
			continue
		}
		if got := kwTree(kwExpr(t, stmts[0])); got != test.want {
			t.Errorf("parsing %q:\ngot  %s\nwant %s", test.stmt, got, test.want)
		}
	}
}

// TestKeywordSelectorTypes checks keyword names in type names (pkg.T).
func TestKeywordSelectorTypes(t *testing.T) {
	f := kwParseFile(t, `package p

import posix "<sys/select.h>"

var m std.map[std.string, int]

func f(std.map) node.type { return nil }

func g(m std.map[int, int], t *node.type) {}

type T struct {
	x.map
	*y.type
}

var rd posix.fd_set
var n = posix.select(fd+1, &rd, nil, nil, nil)
`)
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ValueSpec:
			s := "var " + n.Names[0].Name
			if n.Type != nil {
				s += " " + kwTree(n.Type)
			}
			for _, v := range n.Values {
				s += " = " + kwTree(v)
			}
			got = append(got, s)
		case *ast.Field:
			got = append(got, "field "+kwTree(n.Type))
		}
		return true
	})
	want := []string{
		"var m IndexList(Sel(std, map); Sel(std, string), int)",
		"field Sel(std, map)",
		"field Sel(node, type)",
		"field IndexList(Sel(std, map); int, int)",
		"field Star(Sel(node, type))",
		"field Sel(x, map)",
		"field Star(Sel(y, type))",
		"var rd Sel(posix, fd_set)",
		"var n = Call(Sel(posix, select); Binary(fd + 1), Unary(&rd), nil, nil, nil)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestEveryKeywordSelector(t *testing.T) {
	for _, kw := range kwKeywords {
		// On the same line, the keyword is a name.
		stmts := kwParseBody(t, "_ = x."+kw+"\nx."+kw+"()\n_ = x . "+kw+"\n_ = x./* c */"+kw)
		want := []string{"Sel(x, " + kw + ")", "Call(Sel(x, " + kw + "))", "Sel(x, " + kw + ")", "Sel(x, " + kw + ")"}
		if len(stmts) != len(want) {
			t.Errorf("%s: got %d statements, want %d", kw, len(stmts), len(want))
			continue
		}
		for i, s := range stmts {
			if got := kwTree(kwExpr(t, s)); got != want[i] {
				t.Errorf("%s: statement %d is %s, want %s", kw, i+1, got, want[i])
			}
		}

		// On the next line, it stays a keyword, so an unfinished "x."
		// is still a syntax error (TestIncompleteSelection).
		for _, test := range []struct {
			sep  string
			line int
			col  int
		}{
			{"\n", 4, 1},
			{"\n\t", 4, 2},
			{" // c\n", 4, 1},
			{"/*\n*/", 4, 3},
		} {
			src := "package p\n\nvar _ = x." + test.sep + kw + "\n"
			err := kwFirstError(t, src)
			wantMsg := "expected selector or type assertion, found '" + kw + "'"
			if err.Msg != wantMsg || err.Pos.Line != test.line || err.Pos.Column != test.col {
				t.Errorf("parsing %q: the first error is %v, want test.gp:%d:%d: %s", src, err, test.line, test.col, wantMsg)
			}
		}
	}
}

func TestKeywordSelectorPositions(t *testing.T) {
	const src = "package p\n\nvar _ = std . /* c */ map[k]\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	index := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.IndexExpr)
	sel := index.X.(*ast.SelectorExpr)
	for _, test := range []struct {
		what string
		pos  token.Pos
		want string
	}{
		{"std", sel.X.Pos(), "test.gp:3:9"},
		{"map", sel.Sel.Pos(), "test.gp:3:23"},
		{"the end of map", sel.Sel.End(), "test.gp:3:26"},
		{"[", index.Lbrack, "test.gp:3:26"},
	} {
		if got := fset.Position(test.pos).String(); got != test.want {
			t.Errorf("%s is at %s, want %s", test.what, got, test.want)
		}
	}
	if len(f.Comments) != 1 || len(f.Comments[0].List) != 1 || f.Comments[0].List[0].Text != "/* c */" {
		t.Errorf("got the comments %v, want only /* c */", f.Comments)
	}
}

// TestKeywordSelectorSemicolon checks that a keyword name at the end of a
// line ends the statement, like any name.
func TestKeywordSelectorSemicolon(t *testing.T) {
	stmts := kwParseBody(t, "v := x.range\nw := y.map // c\nz := q.type()\nx.return")
	want := []string{"Sel(x, range)", "Sel(y, map)", "Call(Sel(q, type))", "Sel(x, return)"}
	if len(stmts) != len(want) {
		t.Fatalf("got %d statements, want %d", len(stmts), len(want))
	}
	for i, s := range stmts {
		if got := kwTree(kwExpr(t, s)); got != want[i] {
			t.Errorf("statement %d is %s, want %s", i+1, got, want[i])
		}
	}
}

// TestKeywordAfterDotUnaffected checks Go code that the patch must not
// change: type assertions and type switches, where '(' follows the '.',
// "..." and the '.' of a number, which are not '.' tokens, and dot imports.
func TestKeywordAfterDotUnaffected(t *testing.T) {
	for _, test := range []struct{ stmt, want string }{
		{"_ = x.(map[string]int)", "Assert(x; Map(string; int))"},
		{"_ = x.(func())", "Assert(x; Func)"},
		{"_ = x.(chan int)", "Assert(x; Chan(int))"},
		{"_ = x.(interface{})", "Assert(x; Interface)"},
		{"_ = x.(struct{})", "Assert(x; Struct)"},

		{"_ = [...]struct{}{}", "Lit(Array(Ellipsis(-); Struct))"},
		{"_ = 1. + .5", "Binary(1. + .5)"},
		{"f(xs...)", "Call(f; xs...)"},
	} {
		stmts := kwParseBody(t, test.stmt)
		if got := kwTree(kwExpr(t, stmts[0])); got != test.want {
			t.Errorf("parsing %q:\ngot  %s\nwant %s", test.stmt, got, test.want)
		}
	}
	for _, body := range []string{
		"switch v := x.(type) {\ncase int:\n\t_ = v\n}",
		"switch x. (type) {\n}",
	} {
		stmts := kwParseBody(t, body)
		sw, ok := stmts[0].(*ast.TypeSwitchStmt)
		if !ok {
			t.Errorf("parsing %q: got a %T, want a type switch", body, stmts[0])
			continue
		}
		if got := kwTree(kwExpr(t, sw.Assign)); got != "Assert(x; type)" {
			t.Errorf("parsing %q: the type switch has %s, want Assert(x; type)", body, got)
		}
	}

	// "..." in a parameter list.
	f := kwParseFile(t, "package p\n\nfunc f(a ...map[string]int) {}\n")
	param := f.Decls[0].(*ast.FuncDecl).Type.Params.List[0]
	if got := kwTree(param.Type); got != "Ellipsis(Map(string; int))" {
		t.Errorf("the parameter type is %s, want Ellipsis(Map(string; int))", got)
	}

	// Dot imports.
	for _, src := range []string{
		"package p\n\nimport . \"fmt\"\n",
		"package p\n\nimport (\n\t. \"fmt\"\n\tm \"math\"\n)\n",
	} {
		f := kwParseFile(t, src)
		if imp := f.Imports[0]; imp.Name == nil || imp.Name.Name != "." || imp.Path.Value != `"fmt"` {
			t.Errorf("parsing %q: got import %v %v, want . \"fmt\"", src, imp.Name, imp.Path.Value)
		}
	}
}
