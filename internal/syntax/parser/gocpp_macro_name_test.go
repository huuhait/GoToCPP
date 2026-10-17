// The tests of gocpp's patch 5 (../FORK.md §4): a # directly followed by a
// letter is one name, #Name, in Phase10Syntax mode: a macro use inside code,
// #util.Dbg(x), #func(), #body(). This file is gocpp's own, not upstream's:
// ../patches/0005-macro-names.patch adds it. The helpers are those of
// gocpp_must_test.go (patch 4).

package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// mnTests are inputs with a macro use and the dump of their node.
var mnTests = []dtTest{
	{"expr", `#assert(n > 0)`, `CallExpr{Fun: #assert, Args: [BinaryExpr{X: n, Op: >, Y: 0}]}`, ""},
	{"expr", `#Dbg(x)`, `CallExpr{Fun: #Dbg, Args: [x]}`, ""},
	{"expr", `#util.Dbg(cfg.Port)`, `CallExpr{Fun: #util.Dbg, Args: [cfg.Port]}`, ""},
	{"expr", `#a.b.c(x)`, `CallExpr{Fun: #a.b.c, Args: [x]}`, ""},
	{"expr", `#Dbg[int](x)`, `CallExpr{Fun: IndexExpr{X: #Dbg, Index: int}, Args: [x]}`, ""},
	{"expr", `#util.Dbg[int, string](x)`, `CallExpr{Fun: IndexListExpr{X: #util.Dbg, Indices: [int, string]}, Args: [x]}`, ""},
	{"expr", `#file()`, `CallExpr{Fun: #file}`, ""},
	{"expr", `#line()`, `CallExpr{Fun: #line}`, ""},
	{"expr", `#func()`, `CallExpr{Fun: #func}`, ""},
	{"expr", `#text(x)`, `CallExpr{Fun: #text, Args: [x]}`, ""},
	{"expr", `#body()`, `CallExpr{Fun: #body}`, ""},
	{"expr", `#embed("logo.png")`, `CallExpr{Fun: #embed, Args: ["logo.png"]}`, ""},
	{"expr", `#type()`, `CallExpr{Fun: #type}`, ""},
	{"expr", `#é(x)`, `CallExpr{Fun: #é, Args: [x]}`, ""},
	{"expr", `#_x1(x)`, `CallExpr{Fun: #_x1, Args: [x]}`, ""},
	{"expr", `#Name(x).y`, `CallExpr{Fun: #Name, Args: [x]}.y`, ""},
	{"expr", `#Name(x)[0]`, `IndexExpr{X: CallExpr{Fun: #Name, Args: [x]}, Index: 0}`, ""},
	{"expr", `#Name(x)(y)`, `CallExpr{Fun: CallExpr{Fun: #Name, Args: [x]}, Args: [y]}`, ""},
	{"expr", `#Name(x).Close!()`, `CallExpr{Fun: CallExpr{Fun: #Name, Args: [x]}.Close!}`, ""},
	{"expr", `#Name(#Other(x), F!(y))`, `CallExpr{Fun: #Name, Args: [CallExpr{Fun: #Other, Args: [x]}, CallExpr{Fun: F!, Args: [y]}]}`, ""},
	{"expr", `#a(x) + #b(y)`, `BinaryExpr{X: CallExpr{Fun: #a, Args: [x]}, Op: +, Y: CallExpr{Fun: #b, Args: [y]}}`, ""},
	{"expr", `-#a(x)`, `UnaryExpr{Op: -, X: CallExpr{Fun: #a, Args: [x]}}`, ""},
	{"expr", `#a(x) != #b(y)`, `BinaryExpr{X: CallExpr{Fun: #a, Args: [x]}, Op: !=, Y: CallExpr{Fun: #b, Args: [y]}}`, ""},
	{"expr", "#a(\n\tx,\n)", `CallExpr{Fun: #a, Args: [x]}`, ""},
	{"expr", `[]int{#a(x), 2}`, `CompositeLit{Type: ArrayType{Elt: int}, Elts: [CallExpr{Fun: #a, Args: [x]}, 2]}`, ""},
	{"expr", `func() int { return #Dbg(x) }`, `FuncLit{Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: int}]}}, Body: BlockStmt{List: [ReturnStmt{Results: [CallExpr{Fun: #Dbg, Args: [x]}]}]}}`, ""},
	{"expr", `fmt.Sprintf("%s:%d", #file(), #line())`, `CallExpr{Fun: fmt.Sprintf, Args: ["%s:%d", CallExpr{Fun: #file}, CallExpr{Fun: #line}]}`, ""},
	{"stmt", `#util.Assert(path != "", "path is empty")`, `ExprStmt{X: CallExpr{Fun: #util.Assert, Args: [BinaryExpr{X: path, Op: !=, Y: ""}, "path is empty"]}}`, ""},
	{"stmt", `x := #util.Dbg(cfg.Port)`, `AssignStmt{Lhs: [x], Tok: :=, Rhs: [CallExpr{Fun: #util.Dbg, Args: [cfg.Port]}]}`, ""},
	{"stmt", `#body()`, `ExprStmt{X: CallExpr{Fun: #body}}`, ""},
	{"stmt", `defer #Cleanup()`, `DeferStmt{Call: CallExpr{Fun: #Cleanup}}`, ""},
	{"stmt", `if #check(x) {}`, `IfStmt{Cond: CallExpr{Fun: #check, Args: [x]}, Body: BlockStmt{}}`, ""},
	{"stmt", `for #more() {}`, `ForStmt{Cond: CallExpr{Fun: #more}, Body: BlockStmt{}}`, ""},
	{"stmt", `switch #kind(x) { case #A(): }`, `SwitchStmt{Tag: CallExpr{Fun: #kind, Args: [x]}, Body: BlockStmt{List: [CaseClause{List: [CallExpr{Fun: #A}]}]}}`, ""},
	{"stmt", "#a(x) // c\n", `ExprStmt{X: CallExpr{Fun: #a, Args: [x]}}`, ""},
	{"stmt", "#a(x)\n#b(y)", `ExprStmt{X: CallExpr{Fun: #a, Args: [x]}}`, ""},
}

// TestMacroNames checks the AST of each macro use.
func TestMacroNames(t *testing.T) {
	for _, test := range mnTests {
		if got := muDump(t, test.ctx, test.src, Phase10Syntax|ParseComments); got != test.want {
			t.Errorf("%q:\ngot  %s\nwant %s", test.src, got, test.want)
		}
	}
}

// TestMacroNamesAreOptIn checks that without Phase10Syntax, each of mnTests
// is a syntax error, the one of the stock go/parser (an illegal #).
func TestMacroNamesAreOptIn(t *testing.T) {
	for _, test := range mnTests {
		src := dtSource(t, test)
		for _, mode := range dtModes {
			_, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
			_, gerr := goparser.ParseFile(token.NewFileSet(), "test.gp", src, goparser.Mode(mode))
			if err == nil {
				t.Errorf("%q, mode %v: no error without Phase10Syntax", test.src, mode)
			} else if got, want := dtErrors(err), dtErrors(gerr); !slices.Equal(got, want) {
				t.Errorf("%q, mode %v: the fork reports\n\t%q\nand go/parser\n\t%q", test.src, mode, got, want)
			}
		}
	}
}

// TestMacroNamePositions checks the position and the end of macro names.
func TestMacroNamePositions(t *testing.T) {
	const src = "package p\n\nfunc f() {\n\tx := #util.Dbg(#line())\n\t#body()\n}\n"
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "test.gp", src, Phase10Syntax)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.HasPrefix(id.Name, "#") {
			start, end := fset.Position(id.Pos()), fset.Position(id.End())
			got = append(got, id.Name+" "+src[start.Offset:end.Offset])
		}
		return true
	})
	want := []string{"#util #util", "#line #line", "#body #body"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestMacroNameErrors checks the errors of a macro name where the grammar
// doesn't allow one, and of a # that is no name. A line is a source, and the
// errors "line:col: message".
func TestMacroNameErrors(t *testing.T) {
	const (
		notCalled = " must be called: write "
		tail      = " here: a name with # can only be the start of a macro call"
	)
	for _, tc := range []struct {
		src  string
		want []string
	}{
		// Not called.
		{"package p\nvar _ = #y\n", []string{"2:9: #y" + notCalled + "#y(...)"}},
		{"package p\nvar _ = #util.y\n", []string{"2:9: #util" + notCalled + "#util(...)"}},
		{"package p\nvar _ = #y + 1\n", []string{"2:9: #y" + notCalled + "#y(...)"}},
		{"package p\nvar _ = f(#y)\n", []string{"2:11: #y" + notCalled + "#y(...)"}},
		{"package p\nvar _ = #y[int]\n", []string{"2:9: #y" + notCalled + "#y(...)"}},
		// The name is the start of the selector that is called, with type
		// arguments only at its end.
		{"package p\nvar _ = #a[int].b(1)\n", []string{"2:9: #a" + notCalled + "#a(...)"}},
		{"package p\nvar _ = #a.b[int].c(1)\n", []string{"2:9: #a" + notCalled + "#a(...)"}},
		{"package p\nvar _ = #a.b!(1)\n", []string{"2:9: #a" + notCalled + "#a(...)"}},
		{"package p\nvar _ = #y{}\n", []string{"2:9: #y" + notCalled + "#y(...)"}},
		// As an index (or type argument list) of something else, the name is not called.
		{"package p\nvar _ = x[#a](1)\n", []string{"2:11: #a" + notCalled + "#a(...)"}},
		{"package p\nvar _ = x[1, #a](2)\n", []string{"2:14: #a" + notCalled + "#a(...)"}},
		{"package p\nvar _ = (#y)(x)\n", []string{"2:10: #y" + notCalled + "#y(...)"}},
		{"package p\nvar _ = #y(x)(z) + #w\n", []string{"2:20: #w" + notCalled + "#w(...)"}},
		{"package p\nfunc g() { #y }\n", []string{"2:12: #y" + notCalled + "#y(...)"}},
		{"package p\nfunc g() {\n\tx := #y\n\t_ = x\n}\n", []string{"3:7: #y" + notCalled + "#y(...)"}},
		// A macro name where a name is declared, or used as a type, or
		// after a dot.
		{"package p\nvar #x int\n", []string{"2:5: cannot use #x" + tail}},
		{"package p\nconst #x = 1\n", []string{"2:7: cannot use #x" + tail}},
		{"package p\nvar _ = x.#y(z)\n", []string{"2:11: cannot use #y" + tail}},
		{"package p\nvar _ x.#y\n", []string{"2:9: cannot use #y" + tail}},
		{"package p\nvar _ #T\n", []string{"2:7: cannot use #T" + tail}},
		{"package p\nvar _ []#T\n", []string{"2:9: cannot use #T" + tail}},
		{"package p\ntype #T int\n", []string{"2:6: cannot use #T" + tail}},
		{"package p\nfunc g(#a int) {}\n", []string{"2:8: cannot use #a" + tail}},
		{"package p\nfunc g(a #T) {}\n", []string{"2:10: cannot use #T" + tail}},
		{"package p\nfunc #g() {}\n", []string{"2:6: cannot use #g" + tail}},
		{"package p\nfunc (r #T) g() {}\n", []string{"2:9: cannot use #T" + tail}},
		{"package p\ntype S struct{ #a int }\n", []string{"2:16: cannot use #a" + tail}},
		{"package p\nfunc g() { #L: }\n", []string{"2:12: #L" + notCalled + "#L(...)"}},
		{"package p\nfunc g() { goto #L }\n", []string{"2:17: cannot use #L" + tail}},
		{"package p\nimport #fmt \"fmt\"\n", []string{"2:8: cannot use #fmt" + tail}},
		{"package p\nvar _ = struct{}{}.#y(z)\n", []string{"2:20: cannot use #y" + tail}},
		{"package #p\n", []string{"1:9: cannot use #p" + tail}},
		// A # that is no name.
		{"package p\nvar _ = #\n", []string{"2:9: illegal character U+0023 '#'"}},
		{"package p\nvar _ = # x\n", []string{"2:9: illegal character U+0023 '#'"}},
		{"package p\nvar _ = #(x)\n", []string{"2:9: illegal character U+0023 '#'"}},
		{"package p\nvar _ = ##x(y)\n", []string{"2:9: illegal character U+0023 '#'"}},
		{"package p\nvar _ = #1\n", []string{"2:9: illegal character U+0023 '#'"}},
		// A macro name after a name needs a separator, as any name does.
		{"package p\nvar _ = a #b(c)\n", []string{"2:11: expected ';', found #b"}},
	} {
		got := muErrors(tc.src, Phase10Syntax)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q:\ngot\n\t%s\nwant\n\t%s", tc.src, strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
		}
	}
}

// TestMacroUseInExpr checks ParseExprFrom with a macro use.
func TestMacroUseInExpr(t *testing.T) {
	e, err := ParseExprFrom(token.NewFileSet(), "", "#util.Dbg(x)", Phase10Syntax)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dtDump(e), "CallExpr{Fun: #util.Dbg, Args: [x]}"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := ParseExprFrom(token.NewFileSet(), "", "#y", Phase10Syntax); err == nil || !strings.Contains(err.Error(), "#y must be called") {
		t.Errorf("#y: got error %v", err)
	}
	if _, err := ParseExprFrom(token.NewFileSet(), "", "#y(x)", 0); err == nil {
		t.Errorf("#y(x) without Phase10Syntax: no error")
	}
}

// TestMacroNamesInStrings checks that a # in a string, a rune and a comment
// is not a name, in both modes: the AST is the one of go/parser.
func TestMacroNamesInStrings(t *testing.T) {
	const src = "package p\n\n// #x(y)\nvar a = \"#x(y)\" + `#z` + string('#') /* #x(y) */\n"
	g, err := goparser.ParseFile(token.NewFileSet(), "test.gp", src, goparser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []Mode{ParseComments, ParseComments | Phase10Syntax} {
		f, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
		if err != nil {
			t.Fatal(err)
		}
		if dtDump(f) != dtDump(g) {
			t.Errorf("mode %v: the AST differs from go/parser's:\n%s\n%s", mode, dtDump(f), dtDump(g))
		}
	}
}

// FuzzMacroNames checks, for any input, that the parser doesn't panic in
// Phase10Syntax mode, and that a file without errors has every macro name
// as the start of a call.
func FuzzMacroNames(f *testing.F) {
	for _, test := range mnTests {
		f.Add("package p\n" + dtSource(&testing.T{}, test)[len("package p; "):])
	}
	for _, s := range []string{"package p\nvar _ = #y\n", "package p\nvar #x int\n", "package p\nvar _ = ##x(y)", "package p\nvar _ = #", "package p\nvar _ = #é(x)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		file, err := ParseFile(token.NewFileSet(), "test.gp", src, Phase10Syntax|ParseComments|SkipObjectResolution)
		if err != nil || file == nil {
			return
		}
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			if id, ok := n.(*ast.Ident); ok && strings.HasPrefix(id.Name, "#") && len(id.Name) > 1 && !calledName(stack) {
				t.Errorf("%q: the name %s is not called, and there was no error", src, id.Name)
			}
			return true
		})
	})
}
