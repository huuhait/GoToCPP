package check

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// checkMain loads and checks a module example.com/m with the files (main.gp
// is the root package's file) and returns the program and its main
// package. It fails the test if the program has errors.
func checkMain(t *testing.T, files map[string]string) (*load.Program, *Package) {
	t.Helper()
	files["gocpp.mod"] = "module example.com/m\n"
	root := writeTree(t, files)
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	for _, p := range r.Packages {
		checkScopeTree(t, p)
	}
	return prog, mainPkg(t, prog, r)
}

// checkScopeTree checks that the scope tree of the checked package p and
// its Info agree, which they don't if an expression was type-checked twice
// (types.CheckExpr makes new scopes and objects): each scope of the tree
// below the package scope is in Info.Scopes, once, and each object that a
// name declares or uses in a local scope (not a label) is in a scope of
// the tree, and is what that scope has for its name.
func checkScopeTree(t *testing.T, p *Package) {
	t.Helper()
	tree := make(map[*types.Scope]bool)
	var walk func(s *types.Scope)
	walk = func(s *types.Scope) {
		for i := range s.NumChildren() {
			c := s.Child(i)
			tree[c] = true
			walk(c)
		}
	}
	walk(p.Types.Scope())
	if len(tree) != len(p.Info.Scopes) {
		t.Errorf("%s: %d scopes in the tree, %d in Info.Scopes", p.Load.ImportPath, len(tree), len(p.Info.Scopes))
	}
	for _, s := range p.Info.Scopes {
		if !tree[s] {
			t.Errorf("%s: a scope of Info.Scopes is not in the tree", p.Load.ImportPath)
		}
	}
	for _, m := range []map[*ast.Ident]types.Object{p.Info.Defs, p.Info.Uses} {
		for id, obj := range m {
			if _, ok := obj.(*types.Label); ok || obj == nil || obj.Pkg() != p.Types ||
				obj.Parent() == nil || obj.Parent() == p.Types.Scope() {
				continue // labels have their own scopes, outside the tree
			}
			if !tree[obj.Parent()] {
				t.Errorf("%s: %s is in a scope that is not in the tree", p.Load.ImportPath, id.Name)
			} else if obj.Name() != "_" && obj.Parent().Lookup(obj.Name()) != obj {
				t.Errorf("%s: %s is not the object of its scope", p.Load.ImportPath, id.Name)
			}
		}
	}
}

// labelOf returns how the opaque pass labeled e: "opaque", "typed" (not
// opaque, and go/types recorded a type for e), or "neither". It doesn't
// ask whether the type is complete: the tests check programs without
// errors, and there the sanity rule makes an expression whose type is not
// complete opaque or an internal error.
func labelOf(p *Package, e ast.Expr) string {
	if p.Opaque.IsOpaque(e) {
		return "opaque"
	}
	if _, ok := p.Info.Types[e]; ok {
		return "typed"
	}
	return "neither"
}

// A labelCase is an expression of main.gp and its expected label: the
// expression that types.ExprString prints as src (after the loader's
// rewrite: C.foo for lib.foo), on the first line of the file that contains
// marker, at the first place on that line where at starts.
type labelCase struct {
	marker, at, src, want string
}

// checkLabels checks the labels of the cases in the main package p, whose
// main.gp is src.
func checkLabels(t *testing.T, prog *load.Program, p *Package, src string, cases []labelCase) {
	t.Helper()
	for _, c := range cases {
		e := findExpr(t, prog, p, lineOf(t, src, c.marker), c.at, c.src)
		if got := labelOf(p, e); got != c.want {
			t.Errorf("%s: %s is %s, want %s", c.marker, c.src, got, c.want)
		}
	}
}

// TestOpaqueSpikeFacts checks the facts of the spike that codegen relies on
// (Phase 01 README §2.8): untyped constants passed to C calls keep their
// kind and value, x := lib.foo() gives x the type invalid (so x is opaque),
// and var total int = arr.size() keeps total an int.
func TestOpaqueSpikeFacts(t *testing.T) {
	src := `package main

import (
	"<cstdio>"
	"<lib.h>"
	"<vector>"
)

func main() {
	f := cstdio.fopen("x.txt", "r") // fopen
	cstdio.printf("%d %g\n", 5, 2.0) // printf
	x := lib.foo() // foo
	arr := std.vector[int]{1, 2}
	var total int = arr.size() // size
	_ = total + 1 // total
	_, _ = f, x // use
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	for _, c := range []struct {
		marker, at, src string
		kind            types.BasicKind
		val             constant.Value
	}{
		{"// fopen", `"x.txt"`, `"x.txt"`, types.UntypedString, constant.MakeString("x.txt")},
		{"// fopen", `"r"`, `"r"`, types.UntypedString, constant.MakeString("r")},
		{"// printf", "5", "5", types.UntypedInt, constant.MakeInt64(5)},
		{"// printf", "2.0", "2.0", types.UntypedFloat, constant.MakeFloat64(2)},
	} {
		e := findExpr(t, prog, p, lineOf(t, src, c.marker), c.at, c.src)
		tv := p.Info.Types[e]
		if b, ok := tv.Type.(*types.Basic); !ok || b.Kind() != c.kind || tv.Value == nil || !constant.Compare(tv.Value, token.EQL, c.val) {
			t.Errorf("%s: %s has type %v and value %v, want %v %v", c.marker, c.src, tv.Type, tv.Value, types.Typ[c.kind], c.val)
		}
		if got := labelOf(p, e); got != "typed" {
			t.Errorf("%s: %s is %s, want typed", c.marker, c.src, got)
		}
	}
	for name, want := range map[string]string{"x": "invalid type", "total": "int", "f": "invalid type"} {
		var obj types.Object
		for id, o := range p.Info.Defs {
			if id.Name == name && o != nil {
				obj = o
			}
		}
		if obj == nil || obj.Type().String() != want {
			t.Errorf("%s has type %v, want %s", name, obj, want)
		}
	}
	checkLabels(t, prog, p, src, []labelCase{
		{"// fopen", "cstdio", `C.fopen("x.txt", "r")`, "opaque"},
		{"// foo", "lib", "C.foo()", "opaque"},
		{"// size", "arr", "arr.size()", "opaque"},
		{"// total", "total", "total + 1", "typed"},
		{"// total", "total", "total", "typed"},
		{"// use", "f", "f", "opaque"},
		{"// use", "x", "x", "opaque"},
	})
}

// TestOpaqueRules checks each part of the rule (README §2.8) with a typed
// expression next to the opaque one.
func TestOpaqueRules(t *testing.T) {
	src := `package main

import (
	"fmt"

	"<cstdio>"
	"<lib.h>"
	"<map>"
	"<string>"
	"<vector>"
)

type Point struct{ X, Y int }

func open() *cstdio.FILE { return nil }

func two() (int, *cstdio.FILE) { return 1, nil }

func count() int { return 1 }

type file struct {
	f *cstdio.FILE
	n int
}

type Box[T any] struct{ v T }

type G[T any] struct {
	f *cstdio.FILE
	g func(T) *cstdio.FILE
	v T
}

func main() {
	x := lib.foo() // rooted
	var xs []C.int
	var ys []int
	v := std.vector[int]{1, 2}
	var total int = v.size()
	_ = xs[0] // index-xs
	_ = ys[0] // index-ys
	_ = open() // call-open
	_ = count() // call-count
	g := open()
	_ = g // use-g
	n, p := two()
	_, _ = n, p // use-np
	a, b := std.minmax(1, 2)
	_, _ = a, b // use-ab
	var c = lib.bar()
	_ = c // use-c
	var d, e = lib.pair()
	_, _ = d, e // use-de
	for i, e := range v {
		_, _ = i, e // use-range
	}
	for i, e := range ys {
		_, _ = i, e // use-range-go
	}
	for j := range 3 {
		_ = j // use-j
	}
	func() {
		_, _ = x, total // closure
	}()
	h := func() *cstdio.FILE { return g }
	_ = h() // funclit
	var fl file
	_, _ = fl.f, fl.n // field
	_ = C.Point{1, 2} // lit
	_ = []C.Point{{1, 2}, {X: 3}} // lit-elided
	_ = []Point{{1, 2}, {X: 3}} // lit-go
	_ = std.map[std.string, int]{"a": 1} // lit-map
	_ = map[string]int{"a": 1} // lit-gomap
	v.push_back(3) // method
	_ = v.at(0) + 1 // binary
	_ = total + 1 // binary-go
	_ = v[1:2] // slice
	_ = *g // star
	_ = &v // unary
	_ = -x // neg
	_ = x == nil // cmp
	_ = !lib.ok() // not
	_ = (x) // paren
	var any1 any = 1
	switch s := any1.(type) {
	case *cstdio.FILE:
		_ = s // case-c
	case int:
		_ = s // case-int
	case nil:
		_ = s // case-nil
	}
	var fp *cstdio.FILE = nil // nil
	_ = fp
	var mk map[C.int]int
	_ = mk // map-key
	ps := &xs
	_ = ps // ptr
	var bx Box[[]C.int]
	_, _ = bx, bx.v // box
	var gi G[int]
	_, _, _ = gi.f, gi.g, gi.v // generic-field
	fmt.Println(x, total, len(xs)) // args
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	checkLabels(t, prog, p, src, []labelCase{
		// Rooted in C.
		{"// rooted", "lib", "C.foo()", "opaque"},
		{"// rooted", "lib", "C.foo", "opaque"},
		// An object declared with a type that contains a C type.
		{"// index-xs", "xs", "xs[0]", "opaque"},
		{"// index-xs", "xs", "xs", "opaque"},
		{"// index-xs", "0", "0", "typed"},
		{"// index-ys", "ys", "ys[0]", "typed"},
		{"// field", "fl.f", "fl.f", "opaque"},
		{"// field", "fl.f", "fl", "typed"},
		{"// field", "fl.n", "fl.n", "typed"},
		// A call of a function whose result type contains a C type.
		{"// call-open", "open", "open()", "opaque"},
		{"// call-open", "open", "open", "opaque"},
		{"// call-count", "count", "count()", "typed"},
		{"// funclit", "h", "h()", "opaque"},
		// Variables initialized from opaque values: :=, var, several
		// values from one call, range, a closure.
		{"// use-g", "g", "g", "opaque"},
		{"// use-np", "n", "n", "typed"},
		{"// use-np", "p", "p", "opaque"},
		{"// use-ab", "a", "a", "opaque"},
		{"// use-ab", "b", "b", "opaque"},
		{"// use-c", "c", "c", "opaque"},
		{"// use-de", "d", "d", "opaque"},
		{"// use-de", "e", "e", "opaque"},
		{"// use-range", "i", "i", "opaque"},
		{"// use-range", "e", "e", "opaque"},
		{"// use-range-go", "i", "i", "typed"},
		{"// use-range-go", "e", "e", "typed"},
		{"// use-j", "j", "j", "typed"},
		{"// closure", "x", "x", "opaque"},
		{"// closure", "total", "total", "typed"},
		{"// case-c", "s", "s", "opaque"},
		{"// case-int", "s", "s", "typed"},
		{"// case-nil", "s", "s", "typed"},
		// Composite literals of opaque types, also with the type left out,
		// and their keys.
		{"// lit", "C", "C.Point{…}", "opaque"},
		{"// lit", "C", "C.Point", "opaque"},
		{"// lit", "1", "1", "typed"},
		{"// lit-elided", "[]", "[]C.Point{…}", "opaque"},
		{"// lit-elided", "[]", "[]C.Point", "opaque"},
		{"// lit-elided", "{1", "(ast: <nil>){…}", "opaque"},
		{"// lit-elided", "{X", "(ast: <nil>){…}", "opaque"},
		{"// lit-elided", "X", "X", "opaque"},
		{"// lit-elided", "3", "3", "typed"},
		{"// lit-go", "[]", "[]Point{…}", "typed"},
		{"// lit-go", "{1", "(ast: <nil>){…}", "typed"},
		{"// lit-go", "{X", "(ast: <nil>){…}", "typed"},
		{"// lit-map", `"a"`, `"a"`, "typed"}, // checkSkipped types the key
		{"// lit-map", "std.string", "C.std.string", "opaque"},
		{"// lit-map", "int", "int", "typed"},
		{"// lit-map", "1", "1", "typed"},
		{"// lit-gomap", `"a"`, `"a"`, "typed"},
		// Method calls on opaque values, and operators on opaque operands.
		{"// method", "v", "v.push_back(3)", "opaque"},
		{"// method", "v", "v.push_back", "opaque"},
		{"// method", "v", "v", "opaque"},
		{"// method", "3", "3", "typed"},
		{"// binary", "v", "v.at(0) + 1", "opaque"},
		{"// binary", "1", "1", "typed"},
		{"// binary-go", "total", "total + 1", "typed"},
		{"// slice", "v", "v[1:2]", "opaque"},
		{"// slice", "1", "1", "typed"},
		{"// star", "*", "*g", "opaque"},
		{"// unary", "&", "&v", "opaque"},
		{"// neg", "-", "-x", "opaque"},
		{"// cmp", "x", "x == nil", "opaque"},
		{"// cmp", "nil", "nil", "typed"},
		{"// not", "!", "!C.ok()", "opaque"},
		{"// paren", "(", "(x)", "opaque"},
		{"// nil", "nil", "nil", "typed"},
		{"// map-key", "mk", "mk", "opaque"},
		{"// ptr", "ps", "ps", "opaque"},
		{"// box", "bx", "bx", "opaque"},
		{"// box", "bx.v", "bx.v", "opaque"},
		{"// generic-field", "gi", "gi.f", "opaque"},
		// The field g of G[int] is a new object (its type has T in it):
		// its Origin is the declared field.
		{"// generic-field", "gi.g", "gi.g", "opaque"},
		{"// generic-field", "gi", "gi", "typed"},
		{"// generic-field", "gi.v", "gi.v", "typed"},
		{"// args", "fmt", "fmt.Println(x, total, len(xs))", "typed"},
		{"// args", "x", "x", "opaque"},
		{"// args", "total", "total", "typed"},
		{"// args", "len", "len(xs)", "typed"},
	})
}

// TestOpaqueGenerics checks generic code with opaque values: a generic
// function called with an opaque argument is opaque (go/types can't infer
// T), an explicit instance is typed, and a C type in a generic body is
// opaque there. It also checks struct literals whose keys are field names
// (not labeled): of a type parameter's struct core type, and of a struct
// with a C field.
func TestOpaqueGenerics(t *testing.T) {
	src := `package main

import (
	"<cstdio>"
	"<vector>"
)

func first[T any](xs ...T) T { return xs[0] }

type Box[T any] struct{ v T }

func (b Box[T]) get() T { return b.v }

func keep[T any](x T) T {
	var f *cstdio.FILE
	_ = f // generic-body
	return x // generic-x
}

type pair struct {
	f *cstdio.FILE
	n int
}

func mk[T ~struct{ X int }]() T { return T{X: 1} } // core-struct

func main() {
	v := std.vector[int]{}
	_ = first(v) // first-v
	_ = first[int](1) // first-int
	w := first(v)
	_ = w // use-w
	bx := Box[int]{v: 1}
	_ = bx.get() // box
	_ = keep(v) // keep-v
	_ = keep(1) // keep-int
	_ = pair{f: nil, n: 1} // pair
	_ = mk[struct{ X int }]()
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	checkLabels(t, prog, p, src, []labelCase{
		{"// generic-body", "f", "f", "opaque"},
		{"// generic-x", "x", "x", "typed"},
		{"// first-v", "first", "first(v)", "opaque"},
		{"// first-v", "first", "first", "opaque"}, // no instance
		{"// first-int", "first", "first[int](1)", "typed"},
		{"// use-w", "w", "w", "opaque"},
		{"// box", "bx", "bx.get()", "typed"},
		{"// keep-v", "keep", "keep(v)", "opaque"},
		{"// keep-int", "keep", "keep(1)", "typed"},
		{"// core-struct", "T{", "T{…}", "typed"},
		{"// pair", "pair", "pair{…}", "typed"},
		{"// pair", "nil", "nil", "typed"},
	})
}

// TestOpaqueOtherPackage checks objects of another package: a function
// with a C result, a variable and a field with a C type are opaque in the
// importing package, the rest is typed. The type expressions are kept by
// the package that declares the objects.
func TestOpaqueOtherPackage(t *testing.T) {
	src := `package main

import "example.com/m/lib"

func main() {
	_ = lib.Open() // open
	_ = lib.Out // out
	_ = lib.Count() // count
	var f lib.File
	_, _ = f.F, f.N // fields
	g := lib.Open()
	_ = g // use-g
}
`
	lib := `package lib

import "<cstdio>"

var Out *cstdio.FILE

func Open() *cstdio.FILE { return nil }

func Count() int { return 0 }

type File struct {
	F *cstdio.FILE
	N int
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src, "lib/lib.gp": lib})
	checkLabels(t, prog, p, src, []labelCase{
		{"// open", "lib", "lib.Open()", "opaque"},
		{"// out", "lib", "lib.Out", "opaque"},
		{"// count", "lib", "lib.Count()", "typed"},
		{"// fields", "f.F", "f.F", "opaque"},
		{"// fields", "f.N", "f.N", "typed"},
		{"// use-g", "g", "g", "opaque"},
	})
	var libPkg *Package
	r := Check(prog, Config{})
	for _, q := range r.Packages {
		if q.Load.ImportPath == "example.com/m/lib" {
			libPkg = q
		}
	}
	if libPkg == nil {
		t.Fatal("no package lib")
	}
	out := libPkg.Types.Scope().Lookup("Out")
	if e := libPkg.Opaque.TypeExpr(out); e == nil || types.ExprString(e) != "*C.FILE" {
		t.Errorf("lib: TypeExpr(Out) = %v, want *C.FILE", e)
	}
	// main's own Opaque has only main's objects.
	if e := p.Opaque.TypeExpr(out); e != nil {
		t.Errorf("main: TypeExpr(lib.Out) = %s, want nil", types.ExprString(e))
	}
}

// TestOpaqueTypeExprs checks the type expressions that the pass records
// for the objects declared with C/C++ types, and that it records none for
// the other objects.
func TestOpaqueTypeExprs(t *testing.T) {
	src := `package main

import (
	"<SDL.h>"
	"<cstdio>"
	"<lib.h>"
)

type App struct {
	win *SDL.SDL_Window
	n   int
	pts []C.int
}

type Handle C.int

type Win = *SDL.SDL_Window

type Plain struct{ n int }

type Embed struct{ *SDL.SDL_Window }

const k C.int = 1

var p *cstdio.FILE

var q int

func f(w *SDL.SDL_Window, n int) (r *SDL.SDL_Renderer, err error) { return nil, nil }

func main() {
	x := lib.foo()
	var a App
	var fs [2]*cstdio.FILE
	_, _, _ = x, a, fs
}
`
	_, pkg := checkMain(t, map[string]string{"main.gp": src})
	scope := pkg.Types.Scope()
	app := scope.Lookup("App").Type().Underlying().(*types.Struct)
	sig := scope.Lookup("f").Type().(*types.Signature)
	local := func(name string) types.Object {
		for id, obj := range pkg.Info.Defs {
			if id.Name == name && obj != nil {
				return obj
			}
		}
		t.Fatalf("no object %s", name)
		return nil
	}
	for _, c := range []struct {
		name string
		obj  types.Object
		want string // "" for no type expression
	}{
		{"App.win", app.Field(0), "*C.SDL_Window"},
		{"App.n", app.Field(1), ""},
		{"App.pts", app.Field(2), "[]C.int"},
		{"App", scope.Lookup("App"), "struct {\n\twin *C.SDL_Window\n\tn   int\n\tpts []C.int\n}"},
		{"Handle", scope.Lookup("Handle"), "C.int"},
		{"Win", scope.Lookup("Win"), "*C.SDL_Window"},
		{"Plain", scope.Lookup("Plain"), ""},
		{"Embed.SDL_Window", scope.Lookup("Embed").Type().Underlying().(*types.Struct).Field(0), "*C.SDL_Window"},
		{"k", scope.Lookup("k"), "C.int"},
		{"p", scope.Lookup("p"), "*C.FILE"},
		{"q", scope.Lookup("q"), ""},
		{"f", scope.Lookup("f"), ""},
		{"w", sig.Params().At(0), "*C.SDL_Window"},
		{"n", sig.Params().At(1), ""},
		{"r", sig.Results().At(0), "*C.SDL_Renderer"},
		{"err", sig.Results().At(1), ""},
		{"x", local("x"), ""},
		{"a", local("a"), ""},
		{"fs", local("fs"), "[2]*C.FILE"},
	} {
		got := ""
		if e := pkg.Opaque.TypeExpr(c.obj); e != nil {
			got = exprText(t, e)
		}
		if got != c.want {
			t.Errorf("TypeExpr(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

// exprText prints e as gofmt would (types.ExprString shortens struct
// types).
func exprText(t *testing.T, e ast.Expr) string {
	t.Helper()
	if _, ok := e.(*ast.StructType); !ok {
		return types.ExprString(e)
	}
	var b strings.Builder
	b.WriteString("struct {\n")
	for _, f := range e.(*ast.StructType).Fields.List {
		b.WriteString("\t" + f.Names[0].Name + strings.Repeat(" ", 4-len(f.Names[0].Name)) + types.ExprString(f.Type) + "\n")
	}
	b.WriteString("}")
	return b.String()
}

// TestOpaqueExamples spot-checks the labels in the target programs that
// use headers (TestExamples checks that none of them gets an internal
// error).
func TestOpaqueExamples(t *testing.T) {
	for _, c := range []struct {
		example, goos string
		cases         []labelCase
	}{
		{"cpp-std", "linux", []labelCase{
			{"nums := std.vector[int]{5, 3, 8, 1}", "std", "C.std.vector[int]{…}", "opaque"},
			{"nums := std.vector[int]{5, 3, 8, 1}", "int", "int", "typed"},
			{"nums := std.vector[int]{5, 3, 8, 1}", "5", "5", "typed"},
			{"nums.push_back(4)", "nums", "nums.push_back(4)", "opaque"},
			{"sorted = append(sorted, n)", "sorted", "sorted", "typed"},
			{"sorted = append(sorted, n)", "n)", "n", "opaque"},
			{"lo, hi := std.minmax(3, 1)", "std", "C.std.minmax(3, 1)", "opaque"},
			{`fmt.Println("minmax:", lo, hi)`, "fmt", `fmt.Println("minmax:", lo, hi)`, "typed"},
			{`fmt.Println("minmax:", lo, hi)`, "lo", "lo", "opaque"},
			{`ages["bob"] = 31`, "ages", `ages["bob"]`, "opaque"},
			{`ages["bob"] = 31`, "31", "31", "typed"},
			{"for name, age := range ages", "ages", "ages", "opaque"},
			{"fmt.Println(name, age)", "name", "name", "opaque"},
			{`cs := std.string(gs)`, "gs", "gs", "typed"},
			{`q := p.parent_path() / "archive" / "2026.txt"`, "p", `p.parent_path() / "archive" / "2026.txt"`, "opaque"},
			{`q := p.parent_path() / "archive" / "2026.txt"`, `"archive"`, `"archive"`, "typed"},
		}},
		{"sqlite", "linux", []labelCase{
			{"func printRow(", "C.int", "C.int", "opaque"},
			{"func printRow(", "**C.char", "**C.char", "opaque"},
			{"func printRow(", "unsafe", "unsafe.Pointer", "typed"},
			{`line += C.GoString(cols[i]) + "=" + C.GoString(vals[i])`, "line", "line", "typed"},
			{`line += C.GoString(cols[i]) + "=" + C.GoString(vals[i])`, "cols", "cols[i]", "opaque"},
			{"csql := C.CString(sql)", "sql)", "sql", "typed"},
			{"func exec(db *sqlite3.sqlite3", "*sqlite3", "*C.sqlite3", "opaque"},
			{`exec(db, "CREATE TABLE`, "exec", `exec(db, "CREATE TABLE fruit (name TEXT, qty INTEGER)")`, "typed"},
			{`exec(db, "CREATE TABLE`, "db", "db", "opaque"},
			{"count := 0", "0", "0", "typed"},
		}},
		{"raylib", "linux", []labelCase{
			{"rl.DrawCircleV(rl.Vector2{ball.X, ball.Y}, ball.R, rl.MAROON)", "rl.Vector2", "C.Vector2{…}", "opaque"},
			{"rl.DrawCircleV(rl.Vector2{ball.X, ball.Y}, ball.R, rl.MAROON)", "ball.X", "ball.X", "typed"},
			{"rl.DrawCircleV(rl.Vector2{ball.X, ball.Y}, ball.R, rl.MAROON)", "rl.MAROON", "C.MAROON", "opaque"},
			{`rl.InitWindow(width, height, "gocpp + raylib")`, "width", "width", "typed"},
			{"smoke := std.getenv", "std", `C.std.getenv("GOCPP_SMOKE") != nil`, "opaque"},
			{"frames := run(ball, smoke)", "run", "run(ball, smoke)", "typed"},
			{"frames := run(ball, smoke)", "smoke", "smoke", "opaque"},
		}},
		{"mixed-sources", "linux", []labelCase{
			{"xs := []C.int{4, -2, 9, 7}", "[]", "[]C.int{…}", "opaque"},
			{"xs := []C.int{4, -2, 9, 7}", "-2", "-2", "typed"},
			{"st := checksum.stats_of(", "len", "len(xs)", "typed"},
			{"t.add(w)", "t", "t.add(w)", "opaque"},
			{"t.add(w)", "w", "w", "typed"},
		}},
		{"win32", "windows", []labelCase{
			{"case windows.WM_CREATE:", "windows", "C.WM_CREATE", "opaque"},
			{"hdc := windows.BeginPaint(hwnd, &ps)", "&ps", "&ps", "opaque"},
			{"var smoke = ", "os", `os.Getenv("GOCPP_SMOKE") != ""`, "typed"},
			{"wc.lpfnWndProc = wndProc", "wc", "wc.lpfnWndProc", "opaque"},
			{"wc.lpfnWndProc = wndProc", "wndProc", "wndProc", "opaque"},
		}},
	} {
		t.Run(c.example, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("../../examples", c.example))
			if err != nil {
				t.Fatal(err)
			}
			text, err := os.ReadFile(filepath.Join(dir, "main.gp"))
			if err != nil {
				t.Fatal(err)
			}
			prog, r := loadAndCheck(t, load.Config{GOOS: c.goos, GOARCH: "amd64"}, dir, ".")
			if errs := errorText(t, prog, r, dir, true); errs != "" {
				t.Fatalf("errors:\n%s", errs)
			}
			checkLabels(t, prog, mainPkg(t, prog, r), string(text), c.cases)
		})
	}
}

// TestSanityRule breaks the types of a checked package by hand and checks
// that the opaque pass reports an internal error for an expression that
// has no type and doesn't come from C/C++, only when the package has no
// hard errors, and only for the innermost such expression.
func TestSanityRule(t *testing.T) {
	src := `package main

import "<lib.h>"

type S struct {
	p *lib.T
	n int
}

func main() {
	a, b := 1, 2
	_ = a + b // sum
	_ = lib.foo() + a // c
	_ = a * b // product
	var s S
	_ = s.n + a // typed-c
	_ = make([]C.int, a+b) // make-checked
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	sum := findExpr(t, prog, p, lineOf(t, src, "// sum"), "a", "a + b")
	sumA := findExpr(t, prog, p, lineOf(t, src, "// sum"), "a", "a")
	product := findExpr(t, prog, p, lineOf(t, src, "// product"), "a", "a * b")
	cSum := findExpr(t, prog, p, lineOf(t, src, "// c"), "lib", "C.foo() + a")
	typedC := findExpr(t, prog, p, lineOf(t, src, "// typed-c"), "s", "s.n + a")
	makeSize := findExpr(t, prog, p, lineOf(t, src, "// make-checked"), "a+b", "a + b")
	saved := make(map[ast.Expr]types.TypeAndValue)
	for e, tv := range p.Info.Types {
		saved[e] = tv
	}
	rerun := func(errs []diag.Diagnostic, change func()) string {
		t.Helper()
		p.Info.Types = make(map[ast.Expr]types.TypeAndValue)
		for e, tv := range saved {
			p.Info.Types[e] = tv
		}
		change()
		p.Errors = new(diag.List)
		for _, d := range errs {
			p.Errors.Add(d)
		}
		p.Opaque = findOpaque(prog.Fset, p)
		var b strings.Builder
		for _, d := range p.Errors.Diagnostics() {
			if strings.HasPrefix(d.Msg, "internal error") {
				fmt.Fprintf(&b, "%s:%d:%d: %s\n", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Pos.Column, d.Msg)
			}
		}
		return b.String()
	}
	const hint = " (this is a bug in gocpp: please report it, with this code)"
	sumLine := lineOf(t, src, "// sum")
	sumErr := fmt.Sprintf("main.gp:%d:6: internal error: expression has no type: a + b%s\n", sumLine, hint)
	aErr := fmt.Sprintf("main.gp:%d:6: internal error: expression has no type: a%s\n", sumLine, hint)
	productErr := fmt.Sprintf("main.gp:%d:6: internal error: expression has no type: a * b%s\n", lineOf(t, src, "// product"), hint)
	soft := diag.Diagnostic{Msg: "declared and not used: z", Soft: true, Code: diag.UnusedVar}
	for _, c := range []struct {
		name   string
		errs   []diag.Diagnostic
		change func()
		want   string
	}{
		{"unchanged", nil, func() {}, ""},
		{"missing", nil, func() { delete(p.Info.Types, sum) }, sumErr},
		{"invalid", nil, func() {
			tv := p.Info.Types[sum]
			tv.Type = types.Typ[types.Invalid]
			p.Info.Types[sum] = tv
		}, sumErr},
		{"incomplete", nil, func() {
			tv := p.Info.Types[product]
			tv.Type = types.NewSlice(types.Typ[types.Invalid])
			p.Info.Types[product] = tv
		}, productErr},
		{"innermost", nil, func() { delete(p.Info.Types, sum); delete(p.Info.Types, sumA) }, aErr},
		{"two", nil, func() { delete(p.Info.Types, sum); delete(p.Info.Types, product) }, sumErr + productErr},
		{"from C", nil, func() { delete(p.Info.Types, cSum) }, ""},
		// s comes from C/C++ (S has a C field), but it is typed, and so is
		// s.n: an operand only makes an expression opaque if it is opaque.
		{"typed operand from C", nil, func() { delete(p.Info.Types, typedC) },
			fmt.Sprintf("main.gp:%d:6: internal error: expression has no type: s.n + a%s\n", lineOf(t, src, "// typed-c"), hint)},
		// go/types checks the sizes of make([]C.int, n) itself ([]C.int
		// is a valid type), so they don't inherit the label of the type.
		{"make with a valid type", nil, func() { delete(p.Info.Types, makeSize) },
			fmt.Sprintf("main.gp:%d:20: internal error: expression has no type: a + b%s\n", lineOf(t, src, "// make-checked"), hint)},
		{"soft unused", []diag.Diagnostic{soft}, func() { delete(p.Info.Types, sum) }, sumErr},
		{"hard", []diag.Diagnostic{{Msg: "undefined: z"}}, func() { delete(p.Info.Types, sum) }, ""},
		// InvalidRangeExpr (109) is soft, but leaves the range variables
		// without types.
		{"other soft", []diag.Diagnostic{{Msg: "cannot range over 3", Soft: true, Code: 109}}, func() { delete(p.Info.Types, sum) }, ""},
		{"unused label", []diag.Diagnostic{{Msg: "label L declared and not used", Soft: true, Code: unusedLabel}}, func() { delete(p.Info.Types, sum) }, sumErr},
		{"warning", []diag.Diagnostic{{Msg: "w", Severity: diag.Warning}}, func() { delete(p.Info.Types, sum) }, sumErr},
	} {
		if got := rerun(c.errs, c.change); got != c.want {
			t.Errorf("%s: internal errors:\n%s\nwant:\n%s", c.name, got, c.want)
		}
	}
}

// TestOpaqueConstructs checks a program that uses C/C++ values in many
// kinds of statements and expressions: it gets no internal error, so each
// expression is typed or opaque. Some of them are places where go/types
// doesn't check an expression because of the opaque one next to it.
func TestOpaqueConstructs(t *testing.T) {
	src := `package main

import (
	"unsafe"

	"<SDL.h>"
	"<cstdio>"
	"<lib.h>"
	"<string>"
	"<vector>"
)

const k C.int = 1 // const

const (
	k1 C.int = iota // iota
	k2
)

type Handle C.int

const kh Handle = 1 // const-named

var ga = gb

var gb = lib.foo()

type I interface{ M() *cstdio.FILE }

type W struct {
	*SDL.SDL_Window
	n int
}

type Box[T any] struct{ v T }

func first[T any](xs ...T) T { return xs[0] }

func ret() C.int { return lib.x() }

func main() {
	cv := lib.foo()
	n := 3
	var e any
	_ = cv.(int) // assert
	_ = make([]C.int, n)
	_ = make(C.vec, n) // make
	_ = new(C.int)
	_ = unsafe.Sizeof(cv)
	_ = len(cv)
	_ = append(cv, n)
	delete(cv, n)
	_ = min(cv, n)
	_ = first[C.int](cv)
	_ = struct{ p *C.int }{p: nil}
	switch cv {
	case 1:
	}
	switch {
	case cv == 1:
	}
	if cv {
	}
	for cv {
		break
	}
	switch v := cv.(type) {
	case int:
		_ = v + n
	case *C.FILE:
		_ = v
	}
	x, ok := cv[n]
	cch := make(chan C.int)
	y, ok2 := <-cch
	z, ok3 := e.(*cstdio.FILE)
	defer C.free(nil)
	go lib.run()
	cv++
	cv += n
	cch <- cv
	var _ C.int = lib.x()
	_ = [...]C.int{1, 2}
	_ = map[string]C.int{"a": n}
	_ = []*C.Point{{1, 2}}
	func(p *C.Point) {}(nil)
	var i I
	_ = i.M()
	_, _, _, _, _, _ = x, ok, y, ok2, z, ok3
	_ = k1 + k2
	_ = (*cstdio.FILE).foo
	_ = lib.f(func(a C.int) C.int { return a })
	var arr [2]C.int
	for i, a := range arr {
		_, _ = i, a
	}
	for a := range cch {
		_ = a
	}
	var s std.string
	bx := Box[std.string]{v: s}
	_ = bx.v
	var w W
	_, _ = w.SDL_Window, w.n
	_ = ga
	var big [lib.N]int
	_ = big
	_ = ret()
	select {
	case cv := <-cch:
		_ = cv
	default:
	}
loop:
	for range 2 {
		continue loop
	}
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	checkLabels(t, prog, p, src, []labelCase{
		{"// const", "1", "1", "typed"},
		{"// iota", "iota", "iota", "opaque"},
		{"// const-named", "1", "1", "typed"},
		{"// assert", "int", "int", "typed"},
		{"// make", "n", "n", "typed"},
	})
}

// TestOpaqueInstances checks generic functions called with C/C++ values:
// an instance whose type arguments are not complete (inferred from
// var xs []C.int, also of a stdlib or another package's function, or from
// the type of a variable) is opaque, and so is a generic function that
// go/types couldn't instantiate (an opaque argument, also with some type
// arguments given), while the call of an instance with Go types is typed.
func TestOpaqueInstances(t *testing.T) {
	src := `package main

import (
	"slices"

	"example.com/m/lib"

	"<vector>"
)

func id[T any](x T) T { return x }

func two[T, U any](t T, u U) T { return t }

func main() {
	var xs []C.int
	slices.Sort(xs) // sort
	_ = slices.Contains(xs, 1) // contains
	_ = id(xs) // id
	var g func([]C.int) []C.int = id // value
	_ = g
	_ = lib.Ident(xs) // lib-xs
	cv := std.vector[int]{}
	_ = two[int](1, cv) // partial
	_ = id(cv) // id-cv
	_ = lib.Ident(cv) // lib-cv
	_ = id(1) // id-int
	_ = slices.Contains([]int{1}, 1) // contains-go
	_ = two[int, string](1, "a") // two-go
}
`
	lib := `package lib

func Ident[T any](x T) T { return x }
`
	prog, p := checkMain(t, map[string]string{"main.gp": src, "lib/lib.gp": lib})
	checkLabels(t, prog, p, src, []labelCase{
		{"// sort", "slices", "slices.Sort(xs)", "typed"},
		{"// sort", "slices", "slices.Sort", "opaque"},
		{"// sort", "xs", "xs", "opaque"},
		{"// contains", "slices", "slices.Contains(xs, 1)", "typed"},
		{"// contains", "slices", "slices.Contains", "opaque"},
		{"// id", "id", "id(xs)", "opaque"},
		{"// id", "id", "id", "opaque"},
		{"// value", "id //", "id", "opaque"},
		{"// lib-xs", "lib", "lib.Ident(xs)", "opaque"},
		{"// lib-xs", "lib", "lib.Ident", "opaque"},
		{"// partial", "two", "two[int](1, cv)", "opaque"},
		{"// partial", "two", "two[int]", "opaque"},
		{"// partial", "two", "two", "opaque"},
		{"// partial", "int", "int", "typed"},
		{"// partial", "1", "1", "typed"},
		{"// id-cv", "id", "id(cv)", "opaque"},
		{"// id-cv", "id", "id", "opaque"},
		{"// lib-cv", "lib", "lib.Ident(cv)", "opaque"},
		{"// lib-cv", "lib", "lib.Ident", "opaque"},
		{"// id-int", "id", "id(1)", "typed"},
		{"// id-int", "id", "id", "typed"},
		{"// contains-go", "slices", "slices.Contains([]int{…}, 1)", "typed"},
		{"// contains-go", "slices", "slices.Contains", "typed"},
		{"// two-go", "two", "two[int, string]", "typed"},
		{"// two-go", "two", "two", "typed"},
	})
}

// TestOpaqueSkipped checks the expressions that go/types skips next to an
// opaque one: the T of x.(T), the sizes of make(T, n), the values of
// constants with a C/C++ type, and the keys of opaque literals. checkSkipped
// types them, so an import or a variable used only there counts as used
// (strings, m), and constants keep their value. A key that is a name and a
// value with iota stay opaque, with each expression in them.
func TestOpaqueSkipped(t *testing.T) {
	src := `package main

import (
	"fmt"
	"strings"

	"<lib.h>"
	"<map>"
	"<string>"
)

type S struct{ n int }

type H C.int

const two = 2

const k1 C.int = 1 + 2 // k1

const k2 C.int = len("ab") // k2

const k3 C.int = two // k3

const k4 H = two * 3 // k4

const (
	k5 C.int = iota + 1 // k5
	k6
)

func f(x int) int { return x }

func main() {
	cv := lib.foo()
	n := 3
	xs := []int{1}
	_ = cv.(fmt.Stringer) // stringer
	_ = cv.(strings.Builder) // builder
	_ = cv.(*S) // ptr
	_ = cv.([]int) // slice
	_ = make(C.vec, n+1) // make-sum
	_ = make(C.vec, len(xs)) // make-len
	m := 5
	_ = make(C.vec, m) // make-only
	name := "a"
	_ = std.map[int, int]{n + 4: 2} // key-sum
	_ = std.map[std.string, int]{name + "x": 1} // key-name
	_ = std.map[int, int]{f(n): 1} // key-call
	_ = std.map[int, int]{n: 1} // key-ident
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	checkLabels(t, prog, p, src, []labelCase{
		{"// stringer", "cv", "cv.(fmt.Stringer)", "opaque"},
		{"// stringer", "fmt", "fmt.Stringer", "typed"},
		{"// builder", "strings", "strings.Builder", "typed"},
		{"// ptr", "*S", "*S", "typed"},
		{"// ptr", "S", "S", "typed"},
		{"// slice", "[]", "[]int", "typed"},
		{"// make-sum", "make", "make(C.vec, n + 1)", "opaque"},
		{"// make-sum", "n", "n + 1", "typed"},
		{"// make-sum", "n", "n", "typed"},
		{"// make-len", "len", "len(xs)", "typed"},
		{"// make-only", "m)", "m", "typed"},
		{"// key-sum", "n +", "n + 4", "typed"},
		{"// key-sum", "2", "2", "typed"},
		{"// key-name", "name", `name + "x"`, "typed"},
		{"// key-call", "f(", "f(n)", "typed"},
		{"// key-ident", "n:", "n", "opaque"},
		{"// k1", "1 +", "1 + 2", "typed"},
		{"// k2", "len", `len("ab")`, "typed"},
		{"// k3", "two", "two", "typed"},
		{"// k4", "two", "two * 3", "typed"},
		{"// k5", "iota", "iota + 1", "opaque"},
		{"// k5", "iota", "iota", "opaque"},
		{"// k5", "1 //", "1", "opaque"},
	})
	// The values keep Go's constant types (len returns an int).
	for _, c := range []struct {
		marker, at, src, typ string
		val                  int64
	}{
		{"// k1", "1 +", "1 + 2", "untyped int", 3},
		{"// k2", "len", `len("ab")`, "int", 2},
		{"// k4", "two", "two * 3", "untyped int", 6},
	} {
		e := findExpr(t, prog, p, lineOf(t, src, c.marker), c.at, c.src)
		tv := p.Info.Types[e]
		if tv.Type == nil || tv.Type.String() != c.typ || tv.Value == nil ||
			!constant.Compare(tv.Value, token.EQL, constant.MakeInt64(c.val)) {
			t.Errorf("%s: %s has type %v and value %v, want %s %d", c.marker, c.src, tv.Type, tv.Value, c.typ, c.val)
		}
	}
}

// TestOpaqueSkippedErrors checks that a mistake in an expression that
// go/types skipped next to an opaque one is reported, and that an import
// used nowhere else is still reported.
func TestOpaqueSkippedErrors(t *testing.T) {
	src := `package main

import (
	"os"

	"<lib.h>"
)

func main() {
	cv := lib.foo()
	_ = cv.(Undefined)
	_ = make(C.vec, nn)
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:4:2: \"os\" imported and not used\n" +
		"./main.gp:11:10: undefined: Undefined\n" +
		"./main.gp:12:18: undefined: nn\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
}

// TestOpaqueSkippedOnce checks that checkSkipped doesn't check an
// expression again that go/types checked already: next to a type that is
// valid but not complete ([]C.int), go/types checks the sizes of make and
// the keys of a literal itself, and a second check would give the function
// literals in them new scopes and objects (checkMain checks the scope
// tree). Next to an opaque type it checks them once.
func TestOpaqueSkippedOnce(t *testing.T) {
	src := `package main

import "<lib.h>"

func main() {
	cv := lib.foo()
	_ = make([]C.int, cv+func() int { y := 1; return y }()) // make-lit
	_ = []C.int{cv + func() int { z := 2; return z }(): 1} // key-lit
	_ = make(C.vec, func() int { w := 3; return w }()) // make-skipped
}
`
	prog, p := checkMain(t, map[string]string{"main.gp": src})
	checkLabels(t, prog, p, src, []labelCase{
		{"// make-lit", "y }", "y", "typed"},
		{"// key-lit", "z }", "z", "typed"},
		{"// make-skipped", "func", "(func() int literal)()", "typed"},
		{"// make-skipped", "w }", "w", "typed"},
	})
}

// TestOpaqueSkippedElsewhere checks skipped expressions whose opaque
// neighbour comes from another file (without import "C" in the file of the
// expression) or from another package: they are checked, and what they
// use is used.
func TestOpaqueSkippedElsewhere(t *testing.T) {
	src := `package main

import (
	"fmt"

	"example.com/m/lib"
)

const k lib.H = 1 + 2 // k-lib

func main() {
	n := 1
	_ = make(lib.Vec, n) // make-lib
	// lib.CB.(fmt.Stringer) itself is a false error of go/types ("lib.CB
	// (variable with invalid type) is not an interface"); through a local
	// variable it is opaque.
	cb := lib.CB
	_ = cb.(fmt.Stringer) // assert-lib
}
`
	other := `package main

func g() {
	m := 2
	_ = make(Vec2, m) // make-file
}
`
	types := `package main

import "<v.h>"

type Vec2 = C.vec
`
	lib := `package lib

import "<cstdio>"

type H C.int

type Vec = C.vec

var CB = cstdio.fopen("x.txt", "r")
`
	prog, p := checkMain(t, map[string]string{"main.gp": src, "other.gp": other, "types.gp": types, "lib/lib.gp": lib})
	checkLabels(t, prog, p, src, []labelCase{
		{"// k-lib", "1 +", "1 + 2", "typed"},
		{"// make-lib", "make", "make(lib.Vec, n)", "opaque"},
		{"// make-lib", "n)", "n", "typed"},
		{"// assert-lib", "fmt", "fmt.Stringer", "typed"},
	})
	// other.gp has no import "C".
	var otherSrc ast.Expr
	for _, f := range p.Load.Files {
		if filepath.Base(f.Path) != "other.gp" {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "m" && otherSrc == nil && p.Info.Defs[id] == nil {
				otherSrc = id
			}
			return true
		})
	}
	if otherSrc == nil || labelOf(p, otherSrc) != "typed" {
		t.Errorf("m in make(Vec2, m) of other.gp is %v, want typed", otherSrc)
	}
}

// TestOpaqueSkippedUnchecked checks the skipped expressions that
// CheckExpr can't check: a constant value with iota marks the imports it
// uses (also a dot import), and a key that names a local variable marks
// the variable used. Both stay opaque. A key that is a literal whose type
// was left out stays opaque too. A name iota that is not the predeclared
// one is checked.
func TestOpaqueSkippedUnchecked(t *testing.T) {
	src := `package main

import (
	"strconv"

	"<map>"
	"<vector>"
)

const (
	a C.int = iota + strconv.IntSize // iota
	b
)

func main() {
	k := 1
	_ = std.map[int, int]{k: 1} // key-name
	_ = std.map[std.vector[int], int]{{1, 2}: 3} // key-lit
}
`
	dot := `package main

import (
	. "strconv"

	"<lib.h>"
)

const (
	c C.int = iota + IntSize
)
`
	prog, p := checkMain(t, map[string]string{"main.gp": src, "dot.gp": dot})
	checkLabels(t, prog, p, src, []labelCase{
		{"// iota", "iota", "iota + strconv.IntSize", "opaque"},
		{"// key-name", "k:", "k", "opaque"},
		{"// key-lit", "{1", "(ast: <nil>){…}", "opaque"},
		{"// key-lit", "1,", "1", "opaque"},
		{"// key-lit", "3", "3", "typed"},
	})

	shadow := `package main

import "<lib.h>"

var iota = 5

const k C.int = iota + 1 // shadow

func main() {}
`
	prog, p = checkMain(t, map[string]string{"main.gp": shadow})
	checkLabels(t, prog, p, shadow, []labelCase{
		{"// shadow", "iota +", "iota + 1", "typed"},
	})
}

// TestOpaqueNamedCType checks keyed literals of a named type over a C/C++
// struct (type P C.Point): the literal is typed as P, go/types doesn't
// look at its keys, and they are opaque, also in literals whose type was
// left out and for a type of another package.
func TestOpaqueNamedCType(t *testing.T) {
	src := `package main

import (
	"example.com/m/lib"

	"<geo.h>"
)

type P C.Point

func main() {
	_ = P{X: 1} // p
	_ = []P{{X: 1}} // slice
	_ = map[string]P{"a": {X: 2}} // map
	_ = lib.P{X: 1} // lib
}
`
	lib := `package lib

import "<geo.h>"

type P C.Point
`
	prog, p := checkMain(t, map[string]string{"main.gp": src, "lib/lib.gp": lib})
	checkLabels(t, prog, p, src, []labelCase{
		{"// p", "P", "P{…}", "typed"},
		{"// p", "X", "X", "opaque"},
		{"// p", "1", "1", "typed"},
		{"// slice", "X", "X", "opaque"},
		{"// slice", "{X", "(ast: <nil>){…}", "typed"},
		{"// map", `"a"`, `"a"`, "typed"},
		{"// map", "X", "X", "opaque"},
		{"// lib", "lib", "lib.P{…}", "typed"},
		{"// lib", "X", "X", "opaque"},
	})
	if e := p.Opaque.TypeExpr(p.Types.Scope().Lookup("P")); e == nil || types.ExprString(e) != "C.Point" {
		t.Errorf("TypeExpr(P) = %v, want C.Point", e)
	}
}

// TestOpaqueConversionTypeExpr checks that a variable declared from a
// conversion of an opaque value to a type with a C/C++ part (rewritten to
// gp_cgoh.As[T](x)) gets T as its type expression, and that a variable
// that only comes from such a conversion (a range variable) or whose type
// is complete gets none.
func TestOpaqueConversionTypeExpr(t *testing.T) {
	src := `package main

import "<lib.h>"

func main() {
	cv := lib.foo()
	xs := []C.int(cv)
	var ys = []C.int(cv)
	n := int(cv)
	for _, v := range []C.int(cv) {
		_ = v
	}
	_, _, _ = xs, ys, n
}
`
	_, p := checkMain(t, map[string]string{"main.gp": src})
	if len(p.Conversions) == 0 {
		t.Fatal("no conversions: the test needs the rewrite to gp_cgoh.As")
	}
	for name, want := range map[string]string{"xs": "[]C.int", "ys": "[]C.int", "n": "", "v": ""} {
		var obj types.Object
		for id, o := range p.Info.Defs {
			if id.Name == name && o != nil {
				obj = o
			}
		}
		got := ""
		if e := p.Opaque.TypeExpr(obj); e != nil {
			got = types.ExprString(e)
		}
		if got != want {
			t.Errorf("TypeExpr(%s) = %q, want %q", name, got, want)
		}
	}
}

// TestOpaqueSkippedDotImport checks that a dot import of one file that is
// used only in a skipped expression of that file counts as used, when
// another file of the package (without the dot import) comes first.
func TestOpaqueSkippedDotImport(t *testing.T) {
	a := `package main

import "<lib.h>"

func main() {
	cv := lib.foo()
	_ = make(C.vec, len(cv))
}
`
	b := `package main

import (
	. "strconv"

	"<lib.h>"
)

func g() {
	_ = make(C.vec, IntSize)
}
`
	checkMain(t, map[string]string{"a.gp": a, "b.gp": b})
}
