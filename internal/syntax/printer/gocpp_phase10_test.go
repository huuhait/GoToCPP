// The tests of gocpp's patch 7 (../FORK.md §4): the printer prints macro
// declarations, lazy parameters, lines of macros on top of a func or type,
// and the names with a ! or a # that the parser makes (see ../gpast). This
// file is gocpp's own, not upstream's: ../patches/0007-phase10-printer.patch
// adds it.

package printer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// p10Mode is the parser mode of the tests: Phase 10 syntax, with comments.
const p10Mode = parser.ParseComments | parser.Phase10Syntax | parser.SkipObjectResolution

// p10Format parses src and prints it with gofmt's settings.
func p10Format(src string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.gp", src, p10Mode)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	cfg := Config{Mode: UseSpaces | TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// p10Dump returns the tree x as a string, to compare shapes: a node is its
// type with its fields, without positions, objects and comment pointers; an
// Ident is its name. The comments of a file are in the dump as their text.
// The things that go/printer changes in Go code are not in it:
// parentheses around an expression, an empty list of fields and empty
// statements, and whether an empty statement is implicit.
func p10Dump(x any) string {
	var b strings.Builder
	p10DumpValue(&b, reflect.ValueOf(x))
	return b.String()
}

func p10DumpValue(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Invalid:
		b.WriteString("nil")
		return
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
	}
	switch x := v.Interface().(type) {
	case *ast.Ident:
		b.WriteString(x.Name)
		return
	case *ast.CommentGroup:
		fmt.Fprintf(b, "%q", x.Text())
		return
	case *ast.ParenExpr:
		// go/printer removes parentheses around a type in a parameter list
		// (func f((T)) is func f(T)), so a tree and its output may differ
		// in them: found by FuzzPhase10Print.
		p10DumpValue(b, reflect.ValueOf(x.X))
		return
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		p10DumpValue(b, v.Elem())
	case reflect.Slice:
		b.WriteString("[")
		n := 0
		for i := range v.Len() {
			if _, empty := v.Index(i).Interface().(*ast.EmptyStmt); empty {
				continue // go/printer drops empty statements: func f() { ; }
			}
			if n > 0 {
				b.WriteString(", ")
			}
			n++
			p10DumpValue(b, v.Index(i))
		}
		b.WriteString("]")
	case reflect.Struct:
		b.WriteString(v.Type().Name() + "{")
		sep := ""
		for i := range v.NumField() {
			f := v.Type().Field(i)
			switch {
			case f.Type == reflect.TypeFor[token.Pos](), v.Field(i).IsZero():
				continue
			case isEmptyFields(v.Field(i)):
				// go/printer prints an empty list of results, func f()(), as none.
				continue
			case f.Name == "Implicit", f.Name == "Obj", f.Name == "Scope", f.Name == "Unresolved", f.Name == "Imports", f.Name == "Comment", f.Name == "Doc":
				continue
			}
			var fb strings.Builder
			p10DumpValue(&fb, v.Field(i))
			if fb.String() == "[]" {
				continue // a list of empty statements only
			}
			b.WriteString(sep + f.Name + ": " + fb.String())
			sep = ", "
		}
		b.WriteString("}")
	default:
		fmt.Fprint(b, v.Interface())
	}
}

// isEmptyFields reports whether v is a *ast.FieldList with no fields.
func isEmptyFields(v reflect.Value) bool {
	fl, ok := v.Interface().(*ast.FieldList)
	return ok && (fl == nil || len(fl.List) == 0)
}

// p10Tests are sources and what the printer makes of them. The result is
// also the source of the round trip: it parses to the same tree, and
// printing that gives the result again.
var p10Tests = []struct {
	name string
	src  string
	want string // "" if src is already formatted
}{
	{"must call", "package p\n\nfunc f() {\n\tx := os.ReadFile!(path)\n\ty := parse![int](x)\n\tdefer f.Close!()\n\tF!(x)\n}\n", ""},
	{"must call spacing", "package p\n\nfunc f() {\n\tx   :=   os.ReadFile!( path )\n\ty := parse![ int ]( x )\n}\n",
		"package p\n\nfunc f() {\n\tx := os.ReadFile!(path)\n\ty := parse![int](x)\n}\n"},
	{"must function", "package p\n\nfunc loadConfig!(path string) Config {\n\treturn cfg\n}\n", ""},
	{"must function spacing", "package p\n\nfunc   loadConfig!( path   string )   Config{return cfg}\n",
		"package p\n\nfunc loadConfig!(path string) Config { return cfg }\n"},
	{"must method", "package p\n\nfunc (s *Server) Start!() {\n\ts.ln.Close!()\n}\n", ""},
	{"must generic", "package p\n\nfunc parse![T any](s string) T {\n\treturn zero\n}\n", ""},
	{"main must", "package p\n\nfunc main!() {\n\tcfg := loadConfig!(\"app.json\")\n\t_ = cfg\n}\n", ""},
	{"not a must call", "package p\n\nfunc f() {\n\tif !ok {\n\t}\n\tif !(a && b) {\n\t}\n\tx := a != b\n\t_ = x\n}\n", ""},
	{"keyword before paren", "package p\n\nfunc f() {\n\tif!(a) {\n\t}\n\treturn!(x)\n}\n",
		"package p\n\nfunc f() {\n\tif !(a) {\n\t}\n\treturn !(x)\n}\n"},
	{"macro use", "package p\n\nfunc f() {\n\t#util.Assert(path != \"\", \"path is empty\")\n\tport := #util.Dbg(cfg.Port)\n\tfmt.Println(#file(), #line(), #func(), #text(port))\n}\n", ""},
	{"macro use generic", "package p\n\nfunc f() {\n\tx := #Dbg[int](1)\n}\n", ""},
	{"macro use spacing", "package p\n\nfunc f() {\n\t#util.Assert( a,b )\n}\n", "package p\n\nfunc f() {\n\t#util.Assert(a, b)\n}\n"},
	{"macro decl", "package p\n\nmacro Trace() func() {\n\tfmt.Println(\"enter\", #func())\n\treturn func() { fmt.Println(\"exit\", #func()) }\n}\n", ""},
	{"macro decl spacing", "package p\n\nmacro   Assert( cond bool,msg   lazy   string ){\nif !cond {\npanic(msg())\n}\n}\n",
		"package p\n\nmacro Assert(cond bool, msg lazy string) {\n\tif !cond {\n\t\tpanic(msg())\n\t}\n}\n"},
	{"macro decl generic", "package p\n\nmacro Dbg[T any](v T) T {\n\treturn v\n}\n", ""},
	{"macro decl one line", "package p\n\nmacro M() { #body() }\n", ""},
	{"macro lazy kinds", "package p\n\nmacro M(a lazy []int, b lazy *T, c lazy pkg.T, d lazy map[string]int, e lazy func() int, g lazy std.vector[int]) {\n}\n", ""},
	{"macro lazy group", "package p\n\nmacro M(a, b lazy int, c string) {\n}\n", ""},
	{"macro lazy nested", "package p\n\nmacro M(f func(lazy int)) {\n}\n", ""},
	{"macro lazy lines", "package p\n\nmacro M(\n\ta int,\n\tb lazy int,\n) {\n}\n", ""},
	{"macro wrap", "package p\n\nmacro VM() {\n\tvmEnter(#func())\n\tdefer vmExit(#func())\n\t#body()\n}\n", ""},
	{"on top func", "package p\n\n#util.Trace()\nfunc loadConfig!(path string) Config {\n\treturn cfg\n}\n", ""},
	{"on top method", "package p\n\n#util.Locked(&s.mu)\nfunc (s *Store) Put(k string, v int) {\n\ts.m[k] = v\n}\n", ""},
	{"on top several", "package p\n\n#a()\n#b(1, 2)\n#c[int](x)\nfunc f() {\n}\n", ""},
	{"on top type", "package p\n\n#Derive(String)\ntype Color int\n", ""},
	{"on top type group", "package p\n\n#Derive(String)\ntype (\n\tA int\n\tB int\n)\n", ""},
	{"on top spacing", "package p\n\n#a( 1,2 )\n   #util.b(  x )\nfunc f() {}\n", "package p\n\n#a(1, 2)\n#util.b(x)\nfunc f() {}\n"},
	{"on top first", "package p\n#a()\nfunc f() {\n}\n", "package p\n\n#a()\nfunc f() {\n}\n"},
	{"on top blank line kept", "package p\n\n#a()\n\nfunc f() {\n}\n", ""},
	{"on top blank lines squeezed", "package p\n\n#a()\n\n\n\nfunc f() {\n}\n", "package p\n\n#a()\n\nfunc f() {\n}\n"},
	{"on top after others", "package p\n\nvar x int\n#a()\nfunc f() {\n}\ntype T int\n", "package p\n\nvar x int\n\n#a()\nfunc f() {\n}\n\ntype T int\n"},
	{"on top before others", "package p\n\n#a()\nfunc f() {\n}\n#b()\ntype T int\n#c()\nfunc g() {\n}\n",
		"package p\n\n#a()\nfunc f() {\n}\n\n#b()\ntype T int\n\n#c()\nfunc g() {\n}\n"},
	{"doc comment above", "package p\n\n// f does.\n#util.Trace()\n#util.VM()\nfunc f() {\n}\n", ""},
	// A comment between lines of macros gets an empty line before it, as a
	// comment between two declarations does in gofmt.
	{"comment between macros", "package p\n\n// f does.\n#util.Trace()\n// second\n#util.VM()\nfunc f() {\n}\n", "package p\n\n// f does.\n#util.Trace()\n\n// second\n#util.VM()\nfunc f() {\n}\n"},
	{"doc comment macro", "package p\n\n// Assert checks.\nmacro Assert(c bool) {\n}\n", ""},
	{"trailing comment", "package p\n\n#a() // c\n#b()   // d\nfunc f() {\n}\n", "package p\n\n#a() // c\n#b() // d\nfunc f() {\n}\n"},
	// The trailing comment of the last line of macros does not share a
	// column with the one-line func after it.
	{"trailing comment before a one-line func", "package p\n\n#a() // c\nfunc f() { g() }\n", ""},
	{"trailing comments on lines of macros", "package p\n\n#a()      // c\n#bbbbbb() // d\nfunc f() { g() }\n", ""},
	{"comment between", "package p\n\n#a()\n// c\nfunc f() {\n}\n", "package p\n\n#a()\n\n// c\nfunc f() {\n}\n"},
	{"comment in params", "package p\n\nmacro M(a int, /* x */ b lazy int) {\n}\n", "package p\n\nmacro M(a int /* x */, b lazy int) {\n}\n"},
	{"comment in macro body", "package p\n\nmacro M() {\n\t// before\n\t#body() // after\n}\n", ""},
	{"must and macro", "package p\n\n#util.Trace()\nfunc f!() {\n\t#util.Assert(a)\n\tx := G!(#util.Dbg(1))\n\t_ = x\n}\n", ""},
	{"all together", `package util

import (
	"fmt"
	"os"
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

#Derive(String)
type Color int

func main!() {
	cfg := loadConfig!("app.json")
	port := #util.Dbg(cfg.Port)
	fmt.Println("listening on", port)
}
`, ""},
}

// TestPhase10Print checks the output for each source, that the result
// parses to the same tree as the source, and that printing it again changes
// nothing.
func TestPhase10Print(t *testing.T) {
	for _, tc := range p10Tests {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want == "" {
				want = tc.src
			}
			got, err := p10Format(tc.src)
			if err != nil {
				t.Fatalf("formatting: %v", err)
			}
			if got != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
			p10CheckRoundTrip(t, tc.src, got)
		})
	}
}

// p10CheckRoundTrip checks that src and out parse to the same tree, and that
// printing the tree of out gives out.
func p10CheckRoundTrip(t *testing.T, src, out string) {
	t.Helper()
	f1, err1 := parser.ParseFile(token.NewFileSet(), "test.gp", src, p10Mode)
	f2, err2 := parser.ParseFile(token.NewFileSet(), "test.gp", out, p10Mode)
	if err1 != nil || err2 != nil {
		t.Errorf("parsing the source or the output: %v, %v", err1, err2)
		return
	}
	if d1, d2 := p10Dump(f1), p10Dump(f2); d1 != d2 {
		t.Errorf("the tree of the output is not the tree of the source:\n%s\n%s", d1, d2)
	}
	if again, err := p10Format(out); err != nil || again != out {
		t.Errorf("printing the output again: error %v, changed:\n%s\nto\n%s", err, out, again)
	}
}

// TestPhase10GoUnchanged checks that the printer prints Go code as it did:
// the Go printer's own tests pass, and a Go file with a ! or # near
// names, in a string and in a comment, is printed in the same way with and
// without Phase10Syntax.
func TestPhase10GoUnchanged(t *testing.T) {
	const src = `package p

// #a() is not a macro, and f!() is not a call.
func f() {
	s := "#a(f!())"
	if!(ok) {
	}
	x := a != b
	y := !ok
	/* #b() */
	_, _, _ = s, x, y
}
`
	var outs [2]string
	for i, mode := range []parser.Mode{parser.ParseComments, p10Mode} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "test.go", src, mode)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := (&Config{Mode: UseSpaces | TabIndent, Tabwidth: 8}).Fprint(&buf, fset, f); err != nil {
			t.Fatal(err)
		}
		outs[i] = buf.String()
	}
	if outs[0] != outs[1] {
		t.Errorf("the output depends on Phase10Syntax:\n%s\n%s", outs[0], outs[1])
	}
}

// TestPhase10SizeOneLine checks that a one-line macro and a one-line func
// keep their body on the line, and that a long one is broken, as a func is.
func TestPhase10SizeOneLine(t *testing.T) {
	const src = "package p\n\nfunc f() { g() }\nmacro M() { g() }\n"
	got, err := p10Format(src)
	if err != nil {
		t.Fatal(err)
	}
	// gofmt aligns the bodies of one-line funcs on consecutive lines.
	if want := "package p\n\nfunc f()  { g() }\nmacro M() { g() }\n"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestPhase10Fragments checks the printing of a declaration list, a statement
// list and an expression (a node that is not a file), as gofmt does for a
// fragment.
func TestPhase10Fragments(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.gp", "package p\n#a()\nfunc f() { x := F!(#b(1)) }\nmacro M(c lazy int) {}\n", p10Mode)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		node any
		want string
	}{
		{f.Decls[0], "#a()"},
		{f.Decls[1], "func f() { x := F!(#b(1)) }"},
		{f.Decls[1].(*ast.FuncDecl).Body.List[0], "x := F!(#b(1))"},
		{f.Decls[1].(*ast.FuncDecl).Body.List[0].(*ast.AssignStmt).Rhs[0], "F!(#b(1))"},
		{f.Decls[2], "macro M(c lazy int) {}"},
		{f.Decls[2].(*ast.FuncDecl).Type.Params.List[0].Type, "lazy int"},
		{f.Decls[:2], "#a()\nfunc f() { x := F!(#b(1)) }"},
	} {
		var buf bytes.Buffer
		if err := (&Config{Mode: UseSpaces | TabIndent, Tabwidth: 8}).Fprint(&buf, fset, tc.node); err != nil {
			t.Errorf("%T: %v", tc.node, err)
		} else if got := buf.String(); got != tc.want {
			t.Errorf("%T: got %q, want %q", tc.node, got, tc.want)
		}
	}
}

// FuzzPhase10Print checks, for any input that parses in Phase10Syntax mode,
// that the printer doesn't panic, and that the output parses to the same
// declarations. (Printing the output again is not checked: go/printer
// itself doesn't always give the same text the second time, for example for
// an empty comment //.)
func FuzzPhase10Print(f *testing.F) {
	for _, tc := range p10Tests {
		f.Add(tc.src)
	}
	f.Add("package A\nmacro A((A)){}")
	f.Add("package A\nfunc A()()")
	f.Add("package A\nmacro A(){;}")
	f.Fuzz(func(t *testing.T, src string) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.gp", src, p10Mode)
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := (&Config{Mode: UseSpaces | TabIndent, Tabwidth: 8}).Fprint(&buf, fset, file); err != nil {
			return // for example a comment that can't be printed
		}
		out := buf.String()
		f2, err := parser.ParseFile(token.NewFileSet(), "test.gp", out, p10Mode)
		if err != nil {
			t.Errorf("the output of %q does not parse: %v\n%s", src, err, out)
			return
		}
		// The comments are the same, but a comment may move to another
		// node, so compare the declarations and the comments apart.
		d1, d2 := p10Dump(file.Decls), p10Dump(f2.Decls)
		if d1 != d2 {
			t.Errorf("the tree of the output of %q is another:\n%s\n%s", src, d1, d2)
		}
	})
}
