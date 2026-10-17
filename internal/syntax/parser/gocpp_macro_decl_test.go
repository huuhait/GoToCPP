// The tests of gocpp's patch 6 (../FORK.md §4): macro declarations at
// package level (macro Name(params) { ... }), lazy parameters (msg lazy
// string) and lines of macros on top of a func or type (#Trace(), #Derive(String)).
// This file is gocpp's own, not upstream's: ../patches/0006-macro-decls.patch
// adds it. The helpers are those of gocpp_must_test.go (patch 4).

package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
)

// mdTests are macro declarations and the dump of the FuncDecl. A lazy type
// is a UnaryExpr with the Op ILLEGAL, which a dump leaves out as it does any
// zero field.
var mdTests = []dtTest{
	{"decl", `macro Trace() {}`, `FuncDecl{Name: #Trace, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{}}`, ""},
	{"decl", `macro Assert(cond bool, msg lazy string) { if !cond { panic(msg()) } }`,
		`FuncDecl{Name: #Assert, Type: FuncType{Params: FieldList{List: [Field{Names: [cond], Type: bool}, Field{Names: [msg], Type: UnaryExpr{X: string}}]}}, Body: BlockStmt{List: [IfStmt{Cond: UnaryExpr{Op: !, X: cond}, Body: BlockStmt{List: [ExprStmt{X: CallExpr{Fun: panic, Args: [CallExpr{Fun: msg}]}}]}}]}}`, ""},
	{"decl", `macro Dbg[T any](v T) T { return v }`,
		`FuncDecl{Name: #Dbg, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [T], Type: any}]}, Params: FieldList{List: [Field{Names: [v], Type: T}]}, Results: FieldList{List: [Field{Type: T}]}}, Body: BlockStmt{List: [ReturnStmt{Results: [v]}]}}`, ""},
	{"decl", `macro Trace() func() { return func() {} }`,
		`FuncDecl{Name: #Trace, Type: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: FuncType{Params: FieldList{}}}]}}, Body: BlockStmt{List: [ReturnStmt{Results: [FuncLit{Type: FuncType{Params: FieldList{}}, Body: BlockStmt{}}]}]}}`, ""},
	{"decl", `macro Locked(mu *sync.Mutex) func() { mu.Lock(); return mu.Unlock }`,
		`FuncDecl{Name: #Locked, Type: FuncType{Params: FieldList{List: [Field{Names: [mu], Type: StarExpr{X: sync.Mutex}}]}, Results: FieldList{List: [Field{Type: FuncType{Params: FieldList{}}}]}}, Body: BlockStmt{List: [ExprStmt{X: CallExpr{Fun: mu.Lock}}, ReturnStmt{Results: [mu.Unlock]}]}}`, ""},
	{"decl", `macro VM() { vmEnter(#func()); defer vmExit(#func()); #body() }`,
		`FuncDecl{Name: #VM, Type: FuncType{Params: FieldList{}}, Body: BlockStmt{List: [ExprStmt{X: CallExpr{Fun: vmEnter, Args: [CallExpr{Fun: #func}]}}, DeferStmt{Call: CallExpr{Fun: vmExit, Args: [CallExpr{Fun: #func}]}}, ExprStmt{X: CallExpr{Fun: #body}}]}}`, ""},
	{"decl", `macro Once(o *sync.Once) { o.Do(func() { #body() }) }`,
		`FuncDecl{Name: #Once, Type: FuncType{Params: FieldList{List: [Field{Names: [o], Type: StarExpr{X: sync.Once}}]}}, Body: BlockStmt{List: [ExprStmt{X: CallExpr{Fun: o.Do, Args: [FuncLit{Type: FuncType{Params: FieldList{}}, Body: BlockStmt{List: [ExprStmt{X: CallExpr{Fun: #body}}]}}]}}]}}`, ""},
	// lazy: after the name, with every kind of type, grouped.
	{"decl", `macro M(a, b lazy int) {}`,
		`FuncDecl{Name: #M, Type: FuncType{Params: FieldList{List: [Field{Names: [a, b], Type: UnaryExpr{X: int}}]}}, Body: BlockStmt{}}`, ""},
	{"decl", `macro M(a int, b lazy int, c string) {}`,
		`FuncDecl{Name: #M, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: int}, Field{Names: [b], Type: UnaryExpr{X: int}}, Field{Names: [c], Type: string}]}}, Body: BlockStmt{}}`, ""},
	{"decl", `macro M(a lazy []int, b lazy *T, c lazy pkg.T, d lazy map[string]int, e lazy func() int, f lazy chan int, g lazy std.vector[int]) {}`,
		`FuncDecl{Name: #M, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: UnaryExpr{X: ArrayType{Elt: int}}}, Field{Names: [b], Type: UnaryExpr{X: StarExpr{X: T}}}, Field{Names: [c], Type: UnaryExpr{X: pkg.T}}, Field{Names: [d], Type: UnaryExpr{X: MapType{Key: string, Value: int}}}, Field{Names: [e], Type: UnaryExpr{X: FuncType{Params: FieldList{}, Results: FieldList{List: [Field{Type: int}]}}}}, Field{Names: [f], Type: UnaryExpr{X: ChanType{Dir: SEND|RECV, Value: int}}}, Field{Names: [g], Type: UnaryExpr{X: IndexExpr{X: std.vector, Index: int}}}]}}, Body: BlockStmt{}}`, ""},
	{"decl", `macro M[T any](v lazy T) lazyT {}`,
		`FuncDecl{Name: #M, Type: FuncType{TypeParams: FieldList{List: [Field{Names: [T], Type: any}]}, Params: FieldList{List: [Field{Names: [v], Type: UnaryExpr{X: T}}]}, Results: FieldList{List: [Field{Type: lazyT}]}}, Body: BlockStmt{}}`, ""},
	// lazy is an ordinary name in a func type inside a macro's parameters,
	// and a type named lazy is not a modifier.
	{"decl", `macro M(f func(lazy int)) {}`,
		`FuncDecl{Name: #M, Type: FuncType{Params: FieldList{List: [Field{Names: [f], Type: FuncType{Params: FieldList{List: [Field{Names: [lazy], Type: int}]}}}]}}, Body: BlockStmt{}}`, ""},
	{"decl", `macro M(f func(a lazy int)) {}`,
		`FuncDecl{Name: #M, Type: FuncType{Params: FieldList{List: [Field{Names: [f], Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: lazy}, Field{Type: int}]}}}]}}, Body: BlockStmt{}}`, "error"},
	{"decl", "macro M(\n\ta int,\n\tb lazy int,\n) {}", `FuncDecl{Name: #M, Type: FuncType{Params: FieldList{List: [Field{Names: [a], Type: int}, Field{Names: [b], Type: UnaryExpr{X: int}}]}}, Body: BlockStmt{}}`, ""},
}

// TestMacroDecls checks the AST of each macro declaration. A row with "error"
// as its stock column is not a macro test: it checks an error (below).
func TestMacroDecls(t *testing.T) {
	for _, test := range mdTests {
		if test.stock == "error" {
			continue
		}
		f, err := ParseFile(token.NewFileSet(), "test.gp", dtSource(t, test), Phase10Syntax|ParseComments)
		if err != nil {
			t.Errorf("%q: %v", test.src, err)
			continue
		}
		fd := f.Decls[0].(*ast.FuncDecl)
		if got := dtDump(fd); got != test.want {
			t.Errorf("%q:\ngot  %s\nwant %s", test.src, got, test.want)
		}
		if !gpast.IsMacroDecl(fd) {
			t.Errorf("%q: gpast.IsMacroDecl is false", test.src)
		}
	}
}

// TestMacroDeclShape checks the details that the plan relies on: the
// position of the word macro is the position of the declaration, the name
// has a #, and a func declaration is not a macro.
func TestMacroDeclShape(t *testing.T) {
	const src = "package p\n\n// Assert checks.\nmacro Assert(cond bool, msg lazy string) {}\n\nfunc f() {}\n"
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "test.gp", src, Phase10Syntax|ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	m, fn := f.Decls[0].(*ast.FuncDecl), f.Decls[1].(*ast.FuncDecl)
	if got := fset.Position(m.Pos()); got.Line != 4 || got.Column != 1 {
		t.Errorf("the macro starts at %s, want 4:1 (the word macro)", got)
	}
	if got := src[fset.Position(m.Name.Pos()).Offset:][:6]; got != "Assert" || m.Name.Name != "#Assert" {
		t.Errorf("the name is %q at %q", m.Name.Name, got)
	}
	if m.Doc == nil || m.Doc.Text() != "Assert checks.\n" {
		t.Errorf("macro doc: %v", m.Doc)
	}
	if m.Recv != nil || m.Body == nil {
		t.Errorf("macro: recv %v, body %v", m.Recv, m.Body)
	}
	lz, ok := gpast.Lazy(m.Type.Params.List[1].Type)
	if !ok || dtDump(lz) != "string" {
		t.Errorf("the type of msg is not lazy string: %s", dtDump(m.Type.Params.List[1].Type))
	}
	if got := fset.Position(m.Type.Params.List[1].Type.Pos()); got.Column != 29 || src[got.Offset:got.Offset+4] != "lazy" {
		t.Errorf("lazy string starts at %s", got)
	}
	if got := fset.Position(m.Type.Params.List[1].Type.End()); src[got.Offset-6:got.Offset] != "string" {
		t.Errorf("lazy string ends at %s", got)
	}
	if _, ok := gpast.Lazy(m.Type.Params.List[0].Type); ok {
		t.Errorf("the type bool is lazy")
	}
	if gpast.IsMacroDecl(fn) {
		t.Errorf("func f is a macro")
	}
}

// TestMacroNotInFunc checks that macro is an ordinary name everywhere
// except at package level where a declaration starts: a variable, a type, a
// func, a parameter, a field, a label, a package-level var with that name.
func TestMacroNotInFunc(t *testing.T) {
	for _, src := range []string{
		"package p\nfunc f() {\n\tmacro := 1\n\tmacro++\n\t_ = macro\n}\n",
		"package p\nfunc f(macro int) int { return macro }\n",
		"package p\nfunc macro() {}\nfunc g() { macro() }\n",
		"package p\nvar macro int\n",
		"package p\nvar macro, lazy = 1, 2\n",
		"package p\ntype macro int\n",
		"package p\ntype S struct{ macro int }\n",
		"package p\nfunc f() { macro:\n\tfor {} }\n",
		"package p\nfunc f(lazy int, macro string) {}\n",
		"package p\nfunc f(a lazy.T, b lazy[int]) {}\nfunc g(lazy, int) {}\n",
		"package p\ntype lazy int\nfunc f(a lazy) lazy { return a }\n",
		"package p\nfunc f() {\n\tvar macro lazy\n\tmacro = lazy(1)\n}\n",
	} {
		for _, mode := range []Mode{ParseComments, ParseComments | Phase10Syntax} {
			f, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
			g, gerr := goparser.ParseFile(token.NewFileSet(), "test.gp", src, goparser.Mode(mode&^Phase10Syntax))
			if err != nil || gerr != nil {
				t.Errorf("%q, mode %v: errors %v, go/parser %v", src, mode, err, gerr)
			} else if dtDump(f) != dtDump(g) {
				t.Errorf("%q, mode %v: the AST differs from go/parser's:\n%s\n%s", src, mode, dtDump(f), dtDump(g))
			}
		}
	}
}

// opTest is a source with lines of macros on top, and for each declaration
// its line of macros: "f: #a() #b()".
type opTest struct {
	src  string
	want []string
}

var opTests = []opTest{
	{"#Trace()\nfunc f() {}", []string{"f: #Trace()"}},
	{"#util.Trace()\nfunc f() {}", []string{"f: #util.Trace()"}},
	{"#util.Locked(&s.mu)\nfunc (s *Store) Put(k string) {}", []string{"Put: #util.Locked(&s.mu)"}},
	{"#a()\n#b(1, 2)\n#c[int](x)\nfunc f() {}", []string{"f: #a() #b(1, 2) #c[int](x)"}},
	{"#Derive(String)\ntype Color int", []string{"Color: #Derive(String)"}},
	{"#Derive(String, Equal)\ntype Color int", []string{"Color: #Derive(String, Equal)"}},
	{"#Derive(JSON)\ntype T struct{ A int }", []string{"T: #Derive(JSON)"}},
	{"#Derive(String)\ntype (\n\tA int\n\tB int\n)", []string{": #Derive(String)"}},
	{"#a()\nfunc f() {}\n\nfunc g() {}\n\n#b()\nfunc h() {}", []string{"f: #a()", "g:", "h: #b()"}},
	{"#a()\nfunc f[T any](x T) {}", []string{"f: #a()"}},
	{"#a()\nfunc f!() {}", []string{"f!: #a()"}},
	{"#a()\nfunc main!() {}", []string{"main!: #a()"}},
	{"#a(#func(), F!(x))\nfunc f() {}", []string{"f: #a(#func(), F!(x))"}},
	// Comments and blank lines.
	{"// f does.\n#a()\nfunc f() {}", []string{"f: #a()"}},
	{"#a() // c\nfunc f() {}", []string{"f: #a()"}},
	{"#a()\n\nfunc f() {}", []string{"f: #a()"}},
	{"#a()\n// c\nfunc f() {}", []string{"f: #a()"}},
	{"#a()\n/* c */\n#b()\nfunc f() {}", []string{"f: #a() #b()"}},
	{"#a()\nfunc f() {}\n#b()\ntype T int", []string{"f: #a()", "T: #b()"}},
	{"var x int\n#a()\nfunc f() {}\nvar y int", []string{"f: #a()"}},
	{"#a(\n\tx,\n)\nfunc f() {}", []string{"f: #a( x, )"}},
	{"#a(func() {})\nfunc f() {}", []string{"f: #a(func() {})"}},
}

// opSummary describes the lines of macros of each declaration of f: the name
// of the func, type or the group, and its lines.
func opSummary(fset *token.FileSet, src string, f *ast.File) []string {
	var got []string
	for i, d := range f.Decls {
		if _, ok := gpast.OnTop(d); ok {
			continue
		}
		name := ""
		switch d := d.(type) {
		case *ast.FuncDecl:
			name = d.Name.Name
		case *ast.GenDecl:
			if len(d.Specs) == 1 {
				if ts, ok := d.Specs[0].(*ast.TypeSpec); ok {
					name = ts.Name.Name
				}
			}
			if d.Tok == token.VAR {
				continue
			}
		}
		line := name + ":"
		for _, c := range gpast.OnTopOf(f.Decls, i) {
			line += " " + src[fset.Position(c.Pos()).Offset:fset.Position(c.End()).Offset]
		}
		got = append(got, line)
	}
	return got
}

// TestOnTopMacros checks which lines of macros go with which declaration.
func TestOnTopMacros(t *testing.T) {
	for _, tc := range opTests {
		src := "package p\n\n" + tc.src + "\n"
		fset := token.NewFileSet()
		f, err := ParseFile(fset, "test.gp", src, Phase10Syntax|ParseComments)
		if err != nil {
			t.Errorf("%q: %v", tc.src, err)
			continue
		}
		got := opSummary(fset, src, f)
		// A written call may have a space or a line break that the source
		// text keeps: compare without them.
		norm := func(l []string) []string {
			var out []string
			for _, s := range l {
				out = append(out, strings.Join(strings.Fields(s), " "))
			}
			return out
		}
		if got, want := norm(got), norm(tc.want); !slices.Equal(got, want) {
			t.Errorf("%q:\ngot  %q\nwant %q", tc.src, got, want)
		}
	}
}

// TestOnTopShape checks how a line of macros is stored (see gpast): a
// GenDecl with Tok ILLEGAL at the position of the #, in File.Decls before
// the func or type; and the doc comment above the lines goes to the func or
// type.
func TestOnTopShape(t *testing.T) {
	const src = "package p\n\n// F does.\n#a()\n#util.b(1)\nfunc F() {}\n\n// T is.\n#Derive(String)\ntype T int\n\n// G is.\nfunc G() {}\n"
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "test.gp", src, Phase10Syntax|ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Decls) != 6 {
		t.Fatalf("%d declarations, want 6 (two lines of macros, F, one line of macros, T, G)", len(f.Decls))
	}
	for i, wantPos := range map[int]string{0: "4:1", 1: "5:1", 3: "9:1"} {
		g, ok := f.Decls[i].(*ast.GenDecl)
		if !ok || g.Tok != token.ILLEGAL {
			t.Errorf("decl %d is %T %v, want a GenDecl with Tok ILLEGAL", i, f.Decls[i], g)
			continue
		}
		if got := fset.Position(g.Pos()).String()[len("test.gp:"):]; got != wantPos {
			t.Errorf("decl %d at %s, want %s", i, got, wantPos)
		}
		call, ok := gpast.OnTop(g)
		if !ok {
			t.Errorf("decl %d: gpast.OnTop is false", i)
		} else if call.Pos() != g.Pos() {
			t.Errorf("decl %d: the call starts at %s, the declaration at %s", i, fset.Position(call.Pos()), fset.Position(g.Pos()))
		}
		if g.Doc != nil {
			t.Errorf("decl %d: a line of macros has a Doc", i)
		}
	}
	if doc := f.Decls[2].(*ast.FuncDecl).Doc; doc == nil || doc.Text() != "F does.\n" {
		t.Errorf("the doc of F is %v, want the comment above the lines of macros", doc)
	}
	if doc := f.Decls[4].(*ast.GenDecl).Doc; doc == nil || doc.Text() != "T is.\n" {
		t.Errorf("the doc of T is %v", doc)
	}
	if doc := f.Decls[5].(*ast.FuncDecl).Doc; doc == nil || doc.Text() != "G is.\n" {
		t.Errorf("the doc of G is %v", doc)
	}
	if gpast.OnTopOf(f.Decls, 5) != nil || gpast.OnTopOf(f.Decls, 0) != nil {
		t.Errorf("OnTopOf: G has none, and a line of macros has none")
	}
	if n := len(gpast.OnTopOf(f.Decls, 2)); n != 2 {
		t.Errorf("OnTopOf(F): %d, want 2", n)
	}
	// A declaration after the lines in File.Decls.
	if len(f.Decls) > 0 && f.Decls[len(f.Decls)-1].Pos() < f.Decls[0].Pos() {
		t.Errorf("the declarations are not in source order")
	}
}

// TestMacroDeclErrors checks the errors of macro declarations, lazy
// parameters and lines of macros. A line is a source, and the errors
// "line:col: message".
func TestMacroDeclErrors(t *testing.T) {
	const onTop = "macro calls must be on top of a func or type declaration"
	for _, tc := range []struct {
		src  string
		want []string
	}{
		// lazy.
		{"package p\nmacro M(lazy string) {}\n", []string{`2:9: cannot use lazy as a parameter name: lazy is a modifier, write "name lazy T"`}},
		{"package p\nmacro M(a int, lazy string) {}\n", []string{`2:16: cannot use lazy as a parameter name: lazy is a modifier, write "name lazy T"`}},
		{"package p\nmacro M(lazy) {}\n", []string{`2:9: cannot use lazy as a parameter name: lazy is a modifier, write "name lazy T"`}},
		{"package p\nmacro M(msg lazy) {}\n", []string{"2:17: expected type, found ')'"}},
		{"package p\nmacro M(msg lazy, x int) {}\n", []string{"2:17: expected type, found ','"}},
		{"package p\nmacro M(msg lazy lazy int) {}\n", []string{"2:23: missing ',' in parameter list"}},
		{"package p\nmacro M(msg lazy ...int) {}\n", []string{"2:18: expected type, found '...'"}},
		{"package p\nmacro M[lazy any]() {}\n", nil},
		{"package p\nmacro M[T lazy any]() {}\n", []string{"2:16: missing ',' in parameter list"}},
		{"package p\nmacro M() lazy int {}\n", []string{"2:16: expected ';', found int"}},
		// Not a macro: lazy is an ordinary name in a func and a func type.
		{"package p\nfunc f(msg lazy string) {}\n", []string{"2:17: missing ',' in parameter list"}},
		{"package p\nfunc f(lazy string) {}\n", nil},
		{"package p\nvar _ func(msg lazy string)\n", []string{"2:21: missing ',' in parameter list"}},
		{"package p\nmacro M(f func(msg lazy string)) {}\n", []string{"2:25: missing ',' in parameter list"}},
		{"package p\nmacro M() { f := func(msg lazy string) {} }\n", []string{"2:32: missing ',' in parameter list"}},
		// Macro declarations.
		{"package p\nmacro M()\n", []string{"2:7: missing macro body"}},
		{"package p\nmacro M() int\n", []string{"2:7: missing macro body"}},
		{"package p\nmacro\nfunc f() {}\n", []string{"2:6: expected macro name, found newline"}},
		{"package p\nmacro (r T) M() {}\n", []string{"2:7: expected macro name, found '('"}},
		{"package p\nmacro 1() {}\n", []string{"2:7: expected macro name, found 1"}},
		{"package p\nmacro func() {}\n", []string{"2:7: expected macro name, found 'func'"}},
		{"package p\nmacro M!() {}\n", []string{"2:7: cannot use M! here: a name with ! can only be a called function or method, or a func declaration"}},
		{"package p\nmacro #M() {}\n", []string{"2:7: cannot use #M here: a name with # can only be the start of a macro call"}},
		{"package p\nmacro M()\n{}\n", []string{"3:1: unexpected semicolon or newline before {"}},
		{"package p\nmacro M() {} {}\n", []string{"2:14: expected ';', found '{'"}},
		{"package p\nmacro = 1\n", []string{"2:7: expected macro name, found '='"}},
		{"package p\nmacro.x = 1\n", []string{"2:6: expected macro name, found '.'"}},
		// Lines of macros on top of something else.
		{"package p\n#a()\nvar x int\n", []string{"2:1: " + onTop}},
		{"package p\n#a()\nconst x = 1\n", []string{"2:1: " + onTop}},
		{"package p\n#a()\nimport \"fmt\"\n", []string{"3:1: imports must appear before other declarations"}},
		{"package p\n#a()\nmacro M() {}\n", []string{"2:1: " + onTop}},
		{"package p\n#a()\n", []string{"2:1: " + onTop}},
		{"package p\nfunc f() {}\n#a()\n", []string{"3:1: " + onTop}},
		{"package p\n#a()\n#b()\nvar x int\n", []string{"2:1: " + onTop}},
		{"package p\n#a()\n\n\n#b()\n", []string{"2:1: " + onTop}},
		{"package p\n#a() + 1\nfunc f() {}\n", []string{"2:1: expected a macro call on top of a declaration: #Name(...)"}},
		{"package p\n#a\nfunc f() {}\n", []string{"2:1: expected a macro call on top of a declaration: #Name(...)"}},
		{"package p\n#a()()\nfunc f() {}\n", []string{"2:1: expected a macro call on top of a declaration: #Name(...)"}},
		{"package p\n#a() #b()\nfunc f() {}\n", []string{"2:6: expected ';', found #b"}},
		{"package p\n#a() func f() {}\n", []string{"2:6: expected ';', found 'func'"}},
		{"package p\n#a()\nfunc f() {}\n#b()\n", []string{"4:1: " + onTop}},
		{"package p\n#a()\nBad\nfunc f() {}\n", []string{"3:1: expected declaration, found Bad"}},
		{"package p\n#\nfunc f() {}\n", []string{"2:1: illegal character U+0023 '#'"}},
		{"package p\nfoo()\nfunc f() {}\n", []string{"2:1: expected declaration, found foo"}},
		{"package p\nF!()\nfunc f() {}\n", []string{"2:1: expected declaration, found F!"}},
		{"package p\n#a(\n", []string{"2:5: expected ')', found 'EOF'", "2:5: expected ';', found 'EOF'"}},
	} {
		got := muErrors(tc.src, Phase10Syntax|AllErrors)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q:\ngot\n\t%s\nwant\n\t%s", tc.src, strings.Join(got, "\n\t"), strings.Join(tc.want, "\n\t"))
		}
	}
}

// TestMacroInFuncErrors checks that a macro declaration in a function, a
// func literal or a struct is a syntax error like any other (macro is an
// ordinary name there): the errors and the AST are the stock parser's, also
// for a lazy parameter of a func.
func TestMacroInFuncErrors(t *testing.T) {
	for _, src := range []string{
		"package p\nfunc f() {\n\tmacro M() {}\n}\n",
		"package p\nfunc f() {\n\tmacro M\n}\n",
		"package p\nvar _ = func() {\n\tmacro M() {}\n}\n",
		"package p\ntype T struct {\n\tmacro M() {}\n}\n",
		"package p\nfunc f(msg lazy string) {}\n",
		"package p\nvar _ func(msg lazy string)\n",
		"package p\nfunc f() { var f func(msg lazy string) }\n",
	} {
		for _, mode := range dtModes {
			if msg := dtSameAsStock(src, mode|Phase10Syntax); msg != "" {
				t.Errorf("%q, mode %v: %s", src, mode, msg)
			}
		}
	}
}

// TestMacroDeclsAreOptIn checks that without Phase10Syntax, each macro
// declaration and each line of macros is a syntax error, the same as the
// stock go/parser reports.
func TestMacroDeclsAreOptIn(t *testing.T) {
	var srcs []string
	for _, test := range mdTests {
		srcs = append(srcs, dtSource(t, test))
	}
	for _, tc := range opTests {
		srcs = append(srcs, "package p\n\n"+tc.src+"\n")
	}
	for _, src := range srcs {
		for _, mode := range dtModes {
			f, err := ParseFile(token.NewFileSet(), "test.gp", src, mode)
			g, gerr := goparser.ParseFile(token.NewFileSet(), "test.gp", src, goparser.Mode(mode))
			if err == nil {
				if gerr != nil || dtDump(f) != dtDump(g) {
					t.Errorf("%q, mode %v: no error without Phase10Syntax", src, mode)
				}
				continue
			}
			if got, want := dtErrors(err), dtErrors(gerr); !slices.Equal(got, want) {
				t.Errorf("%q, mode %v: the fork reports\n\t%q\nand go/parser\n\t%q", src, mode, got, want)
			}
		}
	}
}

// TestMacroDeclInfo checks that the plan's whole macro example parses, and
// has the shape that the rewrite needs.
func TestMacroExample(t *testing.T) {
	const src = `package util

import (
	"fmt"
	"os"
	"sync"
)

// Assert panics when cond is false.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Dbg[T any](v T) T {
	fmt.Fprintf(os.Stderr, "[%s:%d] %s = %v\n", #file(), #line(), #text(v), v)
	return v
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

macro Recover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", #func()+":", r)
		}
	}()
	#body()
}

// Load loads.
#util.Trace()
#Recover()
func loadConfig!(path string) Config {
	#util.Assert(path != "", "path is empty")
	data := os.ReadFile!(path)
	return parse(data)
}

#util.Locked(&s.mu)
func (s *Store) Put(k string, v int) {
	s.m[k] = v
}

#Derive(String)
type Color int

func main!() {
	cfg := loadConfig!("app.json")
	port := #util.Dbg(cfg.Port)
	fmt.Println("listening on", port)
}
`
	fset := token.NewFileSet()
	f, err := ParseFile(fset, "util.gp", src, Phase10Syntax|ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var macros, onTop, musts int
	for i, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if gpast.IsMacroDecl(d) {
				macros++
			}
			if gpast.IsMust(d.Name.Name) {
				musts++
			}
			if n := len(gpast.OnTopOf(f.Decls, i)); n > 0 {
				onTop += n
			}
		}
	}
	if macros != 5 || onTop != 3 || musts != 2 {
		t.Errorf("%d macros, %d lines of macros on funcs, %d must funcs; want 5, 3, 2", macros, onTop, musts)
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "loadConfig!" && (fd.Doc == nil || fd.Doc.Text() != "Load loads.\n") {
			t.Errorf("the doc of loadConfig! is %v", fd.Doc)
		}
	}
}

// FuzzMacroDecls checks, for any input, that the parser doesn't panic in
// Phase10Syntax mode with the declarations of this patch, and that every
// macro declaration of a file without errors has a body.
func FuzzMacroDecls(f *testing.F) {
	for _, test := range mdTests {
		f.Add("package p\n" + dtSource(&testing.T{}, test)[len("package p; "):])
	}
	for _, tc := range opTests {
		f.Add("package p\n\n" + tc.src + "\n")
	}
	f.Fuzz(func(t *testing.T, src string) {
		file, err := ParseFile(token.NewFileSet(), "test.gp", src, Phase10Syntax|ParseComments)
		if err != nil || file == nil {
			return
		}
		pending := 0
		for _, d := range file.Decls {
			if _, ok := gpast.OnTop(d); ok {
				pending++
				continue
			}
			if fd, ok := d.(*ast.FuncDecl); ok && gpast.IsMacroDecl(fd) && (fd.Body == nil || pending > 0) {
				t.Errorf("%q: a bad macro declaration without an error", src)
			}
			pending = 0
		}
		if pending > 0 {
			t.Errorf("%q: lines of macros at the end without an error", src)
		}
	})
}
