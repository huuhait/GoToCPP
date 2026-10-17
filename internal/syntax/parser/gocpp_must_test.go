// The tests of gocpp's patch 4 (../FORK.md §4): a name directly followed by
// !( or ![ is one name, F!, in Phase10Syntax mode: a must call (F!(x),
// os.ReadFile!(p), parse![int](s)) or the name of a must function (func f!(),
// func (s *S) Start!()). This file is gocpp's own, not upstream's:
// ../patches/0004-must-names.patch adds it. It imports the stock go/parser,
// to compare the fork with it.

package parser

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/printer"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// muParse parses src in Phase10Syntax mode, with the comments.
func muParse(src string) (*ast.File, error) {
	return ParseFile(token.NewFileSet(), "test.gp", src, Phase10Syntax|ParseComments)
}

// muDump returns the dump (dtDump) of the node of src that the context
// ctx names, see dtTest: "expr", "stmt" or "decl".
func muDump(t *testing.T, ctx, src string, mode Mode) string {
	t.Helper()
	test := dtTest{ctx: ctx, src: src}
	f, err := ParseFile(token.NewFileSet(), "test.gp", dtSource(t, test), mode)
	if err != nil {
		t.Errorf("parsing %q: %v", src, err)
		return ""
	}
	n := dtNode(test, f)
	if n == nil {
		t.Fatalf("parsing %q: no node", src)
	}
	return dtDump(n)
}

// muErrors returns the errors of parsing src in the mode, as "line:col: message".
func muErrors(src string, mode Mode) []string {
	_, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
	var list []string
	for _, e := range dtErrors(err) {
		list = append(list, strings.TrimPrefix(e, "test.gp:"))
	}
	return list
}

// muTests are inputs of Phase 10 and the dump of their node. Without
// Phase10Syntax, each one is a syntax error that the stock go/parser
// reports too (TestMustNamesAreOptIn).
var muTests = []dtTest{
	// Must calls, in the forms of the plan.
	{"expr", `F!(x)`, `CallExpr{Fun: F!, Args: [x]}`, ""},
	{"expr", `F!()`, `CallExpr{Fun: F!}`, ""},
	{"expr", `os.ReadFile!(p)`, `CallExpr{Fun: os.ReadFile!, Args: [p]}`, ""},
	{"expr", `a.b.c.F!(x, y)`, `CallExpr{Fun: a.b.c.F!, Args: [x, y]}`, ""},
	{"expr", `x.Close!()`, `CallExpr{Fun: x.Close!}`, ""},
	{"expr", `parse![int](s)`, `CallExpr{Fun: IndexExpr{X: parse!, Index: int}, Args: [s]}`, ""},
	{"expr", `parse![int, string](s)`, `CallExpr{Fun: IndexListExpr{X: parse!, Indices: [int, string]}, Args: [s]}`, ""},
	{"expr", `pkg.parse![[]int](s)`, `CallExpr{Fun: IndexExpr{X: pkg.parse!, Index: ArrayType{Elt: int}}, Args: [s]}`, ""},
	{"expr", `pkg.parse![std.vector[int]](s)`, `CallExpr{Fun: IndexExpr{X: pkg.parse!, Index: IndexExpr{X: std.vector, Index: int}}, Args: [s]}`, ""},
	{"expr", `g(F!(x), H!(y))`, `CallExpr{Fun: g, Args: [CallExpr{Fun: F!, Args: [x]}, CallExpr{Fun: H!, Args: [y]}]}`, ""},
	{"expr", `F!(G!(x))`, `CallExpr{Fun: F!, Args: [CallExpr{Fun: G!, Args: [x]}]}`, ""},
	{"expr", `F!(x).y`, `CallExpr{Fun: F!, Args: [x]}.y`, ""},
	{"expr", `F!(x)[0]`, `IndexExpr{X: CallExpr{Fun: F!, Args: [x]}, Index: 0}`, ""},
	{"expr", `F!(x)(y)`, `CallExpr{Fun: CallExpr{Fun: F!, Args: [x]}, Args: [y]}`, ""},
	{"expr", `F!(x...)`, `CallExpr{Fun: F!, Args: [x]}`, ""},
	{"expr", `F!(x) + G!(y)`, `BinaryExpr{X: CallExpr{Fun: F!, Args: [x]}, Op: +, Y: CallExpr{Fun: G!, Args: [y]}}`, ""},
	{"expr", `-F!(x)`, `UnaryExpr{Op: -, X: CallExpr{Fun: F!, Args: [x]}}`, ""},
	{"expr", `!F!(x)`, `UnaryExpr{Op: !, X: CallExpr{Fun: F!, Args: [x]}}`, ""},
	{"expr", `a != F!(x)`, `BinaryExpr{X: a, Op: !=, Y: CallExpr{Fun: F!, Args: [x]}}`, ""},
	{"expr", `a && F!(x)`, `BinaryExpr{X: a, Op: &&, Y: CallExpr{Fun: F!, Args: [x]}}`, ""},
	{"expr", `[]int{F!(x), 2}`, `CompositeLit{Type: ArrayType{Elt: int}, Elts: [CallExpr{Fun: F!, Args: [x]}, 2]}`, ""},
	{"expr", `func() int { return F!(x) }`, `FuncLit{Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: int}]}}, Body: BlockStmt{List: [ReturnStmt{Results: [CallExpr{Fun: F!, Args: [x]}]}]}}`, ""},
	{"expr", "F!(\n\tx,\n)", `CallExpr{Fun: F!, Args: [x]}`, ""},
	{"expr", "x.\n\tF!(y)", `CallExpr{Fun: x.F!, Args: [y]}`, ""},
	{"expr", "x . /* c */ F!(y)", `CallExpr{Fun: x.F!, Args: [y]}`, ""},
	{"expr", `é!(x)`, `CallExpr{Fun: é!, Args: [x]}`, ""},
	{"expr", `x.F!(y).G!(z)`, `CallExpr{Fun: CallExpr{Fun: x.F!, Args: [y]}.G!, Args: [z]}`, ""},

	// Statements.
	{"stmt", `F!(x)`, `ExprStmt{X: CallExpr{Fun: F!, Args: [x]}}`, ""},
	{"stmt", `x := F!(a)`, `AssignStmt{Lhs: [x], Tok: :=, Rhs: [CallExpr{Fun: F!, Args: [a]}]}`, ""},
	{"stmt", `a, b := F!(a)`, `AssignStmt{Lhs: [a, b], Tok: :=, Rhs: [CallExpr{Fun: F!, Args: [a]}]}`, ""},
	{"stmt", `x = F![int](a)`, `AssignStmt{Lhs: [x], Tok: =, Rhs: [CallExpr{Fun: IndexExpr{X: F!, Index: int}, Args: [a]}]}`, ""},
	{"stmt", `defer f.Close!()`, `DeferStmt{Call: CallExpr{Fun: f.Close!}}`, ""},
	{"stmt", `go F!()`, `GoStmt{Call: CallExpr{Fun: F!}}`, ""},
	{"stmt", `return F!(x)`, `ReturnStmt{Results: [CallExpr{Fun: F!, Args: [x]}]}`, ""},
	{"stmt", `if F!(x) {}`, `IfStmt{Cond: CallExpr{Fun: F!, Args: [x]}, Body: BlockStmt{}}`, ""},
	{"stmt", `if x := F!(a); x != nil {}`, `IfStmt{Init: AssignStmt{Lhs: [x], Tok: :=, Rhs: [CallExpr{Fun: F!, Args: [a]}]}, Cond: BinaryExpr{X: x, Op: !=, Y: nil}, Body: BlockStmt{}}`, ""},
	{"stmt", `for i := 0; i < F!(n); i++ {}`, `ForStmt{Init: AssignStmt{Lhs: [i], Tok: :=, Rhs: [0]}, Cond: BinaryExpr{X: i, Op: <, Y: CallExpr{Fun: F!, Args: [n]}}, Post: IncDecStmt{X: i, Tok: ++}, Body: BlockStmt{}}`, ""},
	{"stmt", `switch F!(x) { case G!(y): }`, `SwitchStmt{Tag: CallExpr{Fun: F!, Args: [x]}, Body: BlockStmt{List: [CaseClause{List: [CallExpr{Fun: G!, Args: [y]}]}]}}`, ""},
	{"stmt", `x := <-F!(c)`, `AssignStmt{Lhs: [x], Tok: :=, Rhs: [UnaryExpr{Op: <-, X: CallExpr{Fun: F!, Args: [c]}}]}`, ""},
	{"stmt", "F!(x) // c\n", `ExprStmt{X: CallExpr{Fun: F!, Args: [x]}}`, ""},

	// Must functions and methods.
	{"decl", `func f!() {}`, `FuncDecl{Name: f!, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{}}`, ""},
	{"decl", `func f!() T {}`, `FuncDecl{Name: f!, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: T}]}}, Body: BlockStmt{}}`, ""},
	{"decl", `func loadConfig!(path string) Config { return cfg }`, `FuncDecl{Name: loadConfig!, Type: FuncType{Params: FieldList{List: [Field{Names: [path], Type: string}]}, Results: FieldList{List: [Field{Type: Config}]}}, Body: BlockStmt{List: [ReturnStmt{Results: [cfg]}]}}`, ""},
	{"decl", `func (s *Server) Start!() {}`, `FuncDecl{Recv: FieldList{List: [Field{Names: [s], Type: StarExpr{X: Server}}]}, Name: Start!, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{}}`, ""},
	{"decl", `func (Server) Start!()`, `FuncDecl{Recv: FieldList{List: [Field{Type: Server}]}, Name: Start!, Type: FuncType{Params: FieldList{}}}`, ""},
	{"decl", `func parse![T any](s string) T`, `FuncDecl{Name: parse!, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [T], Type: any}]}, Params: FieldList{List: [Field{Names: [s], Type: string}]}, Results: FieldList{List: [Field{Type: T}]}}}`, ""},
	{"decl", `func f!() (cfg Config, err error)`, `FuncDecl{Name: f!, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Names: [cfg], Type: Config}, Field{Names: [err], Type: error}]}}}`, ""},
	{"decl", `func main!() {}`, `FuncDecl{Name: main!, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{}}`, ""},
	{"decl", `func init!() {}`, `FuncDecl{Name: init!, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{}}`, ""},
	{"decl", `func f!() { g!() }`, `FuncDecl{Name: f!, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{List: [ExprStmt{X: CallExpr{Fun: g!}}]}}`, ""},
}

// TestMustNames checks the AST of each Phase 10 input.
func TestMustNames(t *testing.T) {
	for _, test := range muTests {
		if got := muDump(t, test.ctx, test.src, Phase10Syntax|ParseComments); got != test.want {
			t.Errorf("%q:\ngot  %s\nwant %s", test.src, got, test.want)
		}
	}
}

// TestMustNamesAreOptIn checks that without Phase10Syntax each of muTests
// is a syntax error, and the same as the stock go/parser reports.
func TestMustNamesAreOptIn(t *testing.T) {
	for _, test := range muTests {
		src := dtSource(t, test)
		for _, mode := range dtModes {
			f, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
			g, gerr := goparser.ParseFile(token.NewFileSet(), "test.gp", src, goparser.Mode(mode))
			if err == nil {
				t.Errorf("%q, mode %v: no error without Phase10Syntax", test.src, mode)
			} else if got, want := dtErrors(err), dtErrors(gerr); !slices.Equal(got, want) {
				t.Errorf("%q, mode %v: the fork reports\n\t%q\nand go/parser\n\t%q", test.src, mode, got, want)
			}
			if f != nil && g != nil && dtDump(f) != dtDump(g) {
				t.Errorf("%q, mode %v: the ASTs differ:\n%s\n%s", test.src, mode, dtDump(f), dtDump(g))
			}
		}
	}
}

// muGoTests are Go inputs with a ! near a name: every one parses in both
// modes, to the AST of the stock go/parser.
var muGoTests = []dtTest{
	{"expr", `a != b`, `BinaryExpr{X: a, Op: !=, Y: b}`, ""},
	{"expr", `a!=b`, `BinaryExpr{X: a, Op: !=, Y: b}`, ""},
	{"expr", `a != (b)`, `BinaryExpr{X: a, Op: !=, Y: ParenExpr{X: b}}`, ""},
	{"expr", `a!=(b)`, `BinaryExpr{X: a, Op: !=, Y: ParenExpr{X: b}}`, ""},
	{"expr", `a !=[]int{}`, `BinaryExpr{X: a, Op: !=, Y: CompositeLit{Type: ArrayType{Elt: int}}}`, ""},
	{"expr", `!a`, `UnaryExpr{Op: !, X: a}`, ""},
	{"expr", `!(a)`, `UnaryExpr{Op: !, X: ParenExpr{X: a}}`, ""},
	{"expr", `!!a`, `UnaryExpr{Op: !, X: UnaryExpr{Op: !, X: a}}`, ""},
	{"expr", `f(!a)`, `CallExpr{Fun: f, Args: [UnaryExpr{Op: !, X: a}]}`, ""},
	{"expr", `f(a, !b)`, `CallExpr{Fun: f, Args: [a, UnaryExpr{Op: !, X: b}]}`, ""},
	{"expr", `a && !(b)`, `BinaryExpr{X: a, Op: &&, Y: UnaryExpr{Op: !, X: ParenExpr{X: b}}}`, ""},
	{"expr", `a[!b]`, `IndexExpr{X: a, Index: UnaryExpr{Op: !, X: b}}`, ""},
	{"expr", `[]bool{!a, !(b)}`, `CompositeLit{Type: ArrayType{Elt: bool}, Elts: [UnaryExpr{Op: !, X: a}, UnaryExpr{Op: !, X: ParenExpr{X: b}}]}`, ""},
	{"stmt", `if !ok {}`, `IfStmt{Cond: UnaryExpr{Op: !, X: ok}, Body: BlockStmt{}}`, ""},
	{"stmt", `if !(a && b) {}`, `IfStmt{Cond: UnaryExpr{Op: !, X: ParenExpr{X: BinaryExpr{X: a, Op: &&, Y: b}}}, Body: BlockStmt{}}`, ""},
	// A keyword is not a name: if!(a) is if !(a).
	{"stmt", `if!(a) {}`, `IfStmt{Cond: UnaryExpr{Op: !, X: ParenExpr{X: a}}, Body: BlockStmt{}}`, ""},
	{"stmt", `if!a {}`, `IfStmt{Cond: UnaryExpr{Op: !, X: a}, Body: BlockStmt{}}`, ""},
	{"stmt", `if x { return!(y) }`, `IfStmt{Cond: x, Body: BlockStmt{List: [ReturnStmt{Results: [UnaryExpr{Op: !, X: ParenExpr{X: y}}]}]}}`, ""},
	{"stmt", `for!(a) {}`, `ForStmt{Cond: UnaryExpr{Op: !, X: ParenExpr{X: a}}, Body: BlockStmt{}}`, ""},
	{"stmt", `switch!(a) {}`, `SwitchStmt{Tag: UnaryExpr{Op: !, X: ParenExpr{X: a}}, Body: BlockStmt{}}`, ""},
	{"stmt", `switch { case!(b): }`, `SwitchStmt{Body: BlockStmt{List: [CaseClause{List: [UnaryExpr{Op: !, X: ParenExpr{X: b}}]}]}}`, ""},
	{"stmt", `switch x { case!(b): }`, `SwitchStmt{Tag: x, Body: BlockStmt{List: [CaseClause{List: [UnaryExpr{Op: !, X: ParenExpr{X: b}}]}]}}`, ""},
	{"stmt", `x := y; if x!=y {}`, `AssignStmt{Lhs: [x], Tok: :=, Rhs: [y]}`, ""},
	{"stmt", `return !x`, `ReturnStmt{Results: [UnaryExpr{Op: !, X: x}]}`, ""},
	{"stmt", `if x {} else if!(y) {}`, `IfStmt{Cond: x, Body: BlockStmt{}, Else: IfStmt{Cond: UnaryExpr{Op: !, X: ParenExpr{X: y}}, Body: BlockStmt{}}}`, ""},
	// A space, a new line or a comment between the name and the !.
	{"expr", `a !(b)`, ``, "error"},
	{"stmt", "a := b\n!c", `AssignStmt{Lhs: [a], Tok: :=, Rhs: [b]}`, ""},
}

// TestMustGoUnchanged checks that the inputs that are Go parse to the same
// AST with and without Phase10Syntax and with the stock go/parser, and
// that the others are errors in the same way.
func TestMustGoUnchanged(t *testing.T) {
	for _, test := range muGoTests {
		src := dtSource(t, test)
		var want string
		var wantErr []string
		if g, err := goparser.ParseFile(token.NewFileSet(), "test.gp", src, goparser.ParseComments); err == nil {
			want = dtDump(g)
		} else {
			wantErr = dtErrors(err)
		}
		for _, mode := range []Mode{ParseComments, ParseComments | Phase10Syntax} {
			f, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
			if got := dtErrors(err); !slices.Equal(got, wantErr) {
				t.Errorf("%q, mode %v: errors\n\t%q\nwant (go/parser)\n\t%q", test.src, mode, got, wantErr)
			} else if err == nil && dtDump(f) != want {
				t.Errorf("%q, mode %v: the AST differs from go/parser's:\n%s\n%s", test.src, mode, dtDump(f), want)
			}
			if err == nil {
				if got := dtDump(dtNode(test, f)); got != test.want {
					t.Errorf("%q, mode %v:\ngot  %s\nwant %s", test.src, mode, got, test.want)
				}
			}
		}
	}
}

// TestMustNamePositions checks that a must name has the position of its
// text and ends where the text ends, and that the call and the declaration
// around it do too.
func TestMustNamePositions(t *testing.T) {
	const src = "package p\n\nfunc (s *S) Start!() {\n\tx := os.ReadFile!(p)\n}\n"
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "test.gp", src, Phase10Syntax)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && strings.HasSuffix(id.Name, "!") {
			start, end := fset.Position(id.Pos()), fset.Position(id.End())
			got = append(got, fmt.Sprintf("%s %d:%d-%d:%d %q", id.Name, start.Line, start.Column, end.Line, end.Column, src[start.Offset:end.Offset]))
		}
		return true
	})
	want := []string{`Start! 3:13-3:19 "Start!"`, `ReadFile! 4:10-4:19 "ReadFile!"`}
	if !slices.Equal(got, want) {
		t.Errorf("got\n\t%s\nwant\n\t%s", strings.Join(got, "\n\t"), strings.Join(want, "\n\t"))
	}
}

// TestMustErrors checks the errors of a name with a ! where the grammar
// doesn't allow one. A line is a source, and the errors "line:col: message".
func TestMustErrors(t *testing.T) {
	const (
		notCalled = " must be called: write "
		here      = "cannot use "
		tail      = " here: a name with ! can only be a called function or method, or a func declaration"
	)
	for _, tc := range []struct {
		src  string
		want []string
	}{
		// Type arguments without a call.
		{"package p\nvar _ = F![int]\n", []string{"2:9: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = os.F![int, string]\n", []string{"2:12: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = (F!)(x)\n", []string{"2:11: expected ')', found '!'"}},
		{"package p\nvar _ = F![int].x\n", []string{"2:9: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = F![int][0]\n", []string{"2:9: F!" + notCalled + "F!(...)"}},
		// The name must be the function that is called, not the start of a
		// longer selector or the X of a selector.
		{"package p\nvar _ = F![int].x(1)\n", []string{"2:9: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = a.F![int].x(1)\n", []string{"2:11: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = a.F![int, int].x(1)\n", []string{"2:11: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = a.F![int][0](1)\n", []string{"2:11: F!" + notCalled + "F!(...)"}},
		{"package p\nvar _ = a.F![int].x.y(1)\n", []string{"2:11: F!" + notCalled + "F!(...)"}},
		// A name with a ! as a variable, field, parameter or type name.
		{"package p\nvar f!(int)\n", []string{"2:5: " + here + "f!" + tail}},
		{"package p\nvar a, f!(int)\n", []string{"2:8: " + here + "f!" + tail}},
		{"package p\ntype T struct{ a!(int) }\n", []string{"2:16: " + here + "a!" + tail}},
		{"package p\nfunc g(a!(b)) {}\n", []string{"2:8: " + here + "a!" + tail}},
		{"package p\nvar _ pkg.T!(int)\n", []string{"2:11: " + here + "T!" + tail}},
		{"package p\ntype I interface{ m!() }\n", []string{"2:19: " + here + "m!" + tail}},
		{"package p\nfunc g() { L!(x): }\n", []string{"2:17: illegal label declaration"}},
		{"package p\nfunc g[T!(any)]() {}\n", []string{"2:8: " + here + "T!" + tail}},
		{"package p\ntype F!(int) int\n", []string{"2:6: " + here + "F!" + tail}},
		// A ! that is not directly after a name stays an error of Go.
		{"package p\nvar _ = f !(x)\n", []string{"2:11: expected ';', found '!'"}},
		{"package p\nfunc g() { fns[i]!() }\n", []string{"2:18: expected ';', found '!'"}},
		{"package p\nfunc g() { f()!() }\n", []string{"2:15: expected ';', found '!'"}},
		{"package p\nfunc g() { F! () }\n", []string{"2:13: expected ';', found '!'"}},
		{"package p\nfunc g() { F!\n(x) }\n", []string{"2:13: expected ';', found '!'", "3:7: expected '}', found 'EOF'"}},
		{"package p\nfunc g() { F!{x} }\n", []string{"2:13: expected ';', found '!'"}},
		{"package p\nfunc F! () {}\n", []string{"2:7: expected '(', found '!'"}},
		{"package p\nfunc F!\n() {}\n", []string{"2:7: expected '(', found '!'", "3:2: expected type, found ')'"}},
		{"package p\nfunc (s S) F!{}\n", []string{"2:13: expected '(', found '!'"}},
	} {
		got := muErrors(tc.src, Phase10Syntax)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q:\ngot\n\t%s\nwant\n\t%s", tc.src, strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
		}
	}
}

// TestMustExpr checks that ParseExprFrom understands a must call, and
// checks the names in an expression too.
func TestMustExpr(t *testing.T) {
	e, err := ParseExprFrom(token.NewFileSet(), "", "os.ReadFile!(p)", Phase10Syntax)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dtDump(e), "CallExpr{Fun: os.ReadFile!, Args: [p]}"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := ParseExprFrom(token.NewFileSet(), "", "F![int]", Phase10Syntax); err == nil || !strings.Contains(err.Error(), "F! must be called") {
		t.Errorf("F![int]: got error %v", err)
	}
	if _, err := ParseExprFrom(token.NewFileSet(), "", "F!(x)", 0); err == nil {
		t.Errorf("F!(x) without Phase10Syntax: no error")
	}
}

// TestMustPrintsLikeGo checks that go/printer prints each Phase 10 call and
// must function as it was written, with the fork's tree: the printer has no
// need to know the names. (Spike of 2026-10-13; the printer fork is only
// for macro, lazy and on-top macros.)
func TestMustPrintsLikeGo(t *testing.T) {
	const src = `package p

func loadConfig!(path string) Config {
	data := os.ReadFile!(path)
	cfg := parse![Config](data)
	defer f.Close!()
	return cfg
}

func (s *Server) Start!() {
	s.ln.Close!()
}
`
	f, err := muParse(src)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := printer.Fprint(&b, token.NewFileSet(), f); err != nil {
		t.Fatal(err)
	}
	// The positions are of another file set, so the layout may differ in
	// blank lines only; compare the lines without the empty ones.
	norm := func(s string) string {
		return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\n' }), "\n")
	}
	if got, want := norm(b.String()), norm(src); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// FuzzMustNames checks, for any input, that the parser doesn't panic in
// Phase10Syntax mode, and that a file without errors has every name with a !
// as the function of a call or as the name of a func declaration.
func FuzzMustNames(f *testing.F) {
	for _, test := range append(slices.Clone(muTests), muGoTests...) {
		f.Add("package p\n" + dtSource(&testing.T{}, test)[len("package p; "):])
	}
	for _, s := range []string{"package p\nvar _ = F![int]\n", "package p\nfunc f!() {}\n", "package p\nvar x = a!=b", "package p\nvar x = a!"} {
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
			if id, ok := n.(*ast.Ident); ok && strings.HasSuffix(id.Name, "!") && len(id.Name) > 1 && !calledName(stack) {
				t.Errorf("%q: the name %s is not called, and there was no error", src, id.Name)
			}
			return true
		})
	})
}
