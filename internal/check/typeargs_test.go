package check

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// lineOf returns the number of the first line of src that contains marker.
func lineOf(t *testing.T, src, marker string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, marker) {
			return i + 1
		}
	}
	t.Fatalf("no line with %q", marker)
	return 0
}

// TestOpaqueArgUses checks that a local variable or an import used only
// in an opaque argument in a type is used: go/types' "declared and not
// used" and "imported and not used" are dropped, also for the variable of
// a type switch, and for an import after an error in the argument (which
// CheckExpr stops at). The variables and imports that are not used are
// still reported.
func TestOpaqueArgUses(t *testing.T) {
	src := `package main

import (
	"os"
	"strings"
	"unsafe"

	"<array>"
	"<vector>"
)

func main() {
	var arr [4]int
	var v std.array[int, len(arr)]
	x := 3
	var w std.array[int, unsafe.Sizeof(x)]
	var a any = 1
	switch y := a.(type) {
	case int:
		var z std.array[int, unsafe.Sizeof(y)]
		_ = z
	}
	unused := 1
	_, _ = v, w
	var e std.vector[func(Pointt) strings.Builder]
	var s std.vector[struct {
		a Aa
		b Bb
	}]
	_, _ = e, s
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	// CheckExpr stops at the first error of an argument: Bb is not
	// reported.
	want := "# example.com/m\n" +
		"./main.gp:4:2: \"os\" imported and not used\n" +
		"./main.gp:23:2: declared and not used: unused\n" +
		"./main.gp:25:24: undefined: Pointt\n" +
		"./main.gp:27:5: undefined: Aa\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
}

// TestOpaqueArgSizes pins unsafe.Sizeof, Alignof and Offsetof in the
// expressions that types.CheckExpr checks (opaque arguments and skipped
// roots): CheckExpr computes them with Go's sizes, so each one is computed
// again with gocppSizes. Where the two agree (an int, an int8 field at
// offset 8), the argument keeps its value; where they differ (a string is
// 16 bytes in Go and 32 in gocpp), it is an error that says to use a
// constant. A constant gets gocpp's value, also in an argument.
func TestOpaqueArgSizes(t *testing.T) {
	src := `package main

import (
	"unsafe"
	. "unsafe"

	"<array>"
	"<stdio.h>"
)

type pair struct {
	a int8
	b string
}

type tri struct {
	s string
	b int8
}

type outer struct {
	x int8
	tri
}

var s string
var p pair
var q *tri
var o outer
var n int

const sizeS = unsafe.Sizeof(s)

var a1 std.array[byte, unsafe.Sizeof(s)]
var a2 std.array[byte, (unsafe.Sizeof(p) + 1)]
var a3 std.array[byte, Sizeof(s)]
var a4 std.array[byte, unsafe.Offsetof(q.b)]
var a5 std.array[byte, unsafe.Offsetof(o.b)]
var a6 std.array[[unsafe.Sizeof(s)]byte, 2]

var b1 std.array[byte, sizeS]
var b2 std.array[byte, unsafe.Sizeof(n)]
var b3 std.array[byte, unsafe.Offsetof(p.b)]
var b4 std.array[byte, unsafe.Alignof(s)]
var b5 std.array[byte, unsafe.Alignof(p)]

const k C.int = unsafe.Sizeof(s)

func main() { _, _, _, _, _, _, _, _, _, _, _ = a1, a2, a3, a4, a5, a6, b1, b2, b3, b4, b5 }
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	// The errors: the line (by a marker), the column, the call, where it is,
	// Go's value and gocpp's.
	wantErrs := []struct {
		marker         string
		col            int
		call           string
		where          string
		goSize, gpSize int
	}{
		{"var a1", 24, "unsafe.Sizeof(s)", "in a C++ template argument", 16, 32},
		{"var a2", 25, "unsafe.Sizeof(p)", "in a C++ template argument", 24, 40},
		{"var a3", 24, "Sizeof(s)", "in a C++ template argument", 16, 32},
		{"var a4", 24, "unsafe.Offsetof(q.b)", "in a C++ template argument", 16, 32},
		{"var a5", 24, "unsafe.Offsetof(o.b)", "in a C++ template argument", 24, 40},
		{"var a6", 19, "unsafe.Sizeof(s)", "in a C++ template argument", 16, 32},
		{"const k C.int", 17, "unsafe.Sizeof(s)", "next to a C/C++ expression", 16, 32},
	}
	want := "# example.com/m\n"
	for _, e := range wantErrs {
		want += fmt.Sprintf("./main.gp:%d:%d: cannot use %s %s: it would be Go's %d, not gocpp's %d (use a constant: const n = %s)\n",
			lineOf(t, src, e.marker), e.col, e.call, e.where, e.goSize, e.gpSize, e.call)
	}
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Fatalf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	for _, c := range []struct {
		at, src string
		want    int64
	}{
		{"sizeS]", "sizeS", 32},
		{"unsafe.Sizeof(n)]", "unsafe.Sizeof(n)", 8},
		{"unsafe.Offsetof(p.b)]", "unsafe.Offsetof(p.b)", 8},
		{"unsafe.Alignof(s)]", "unsafe.Alignof(s)", 8},
		{"unsafe.Alignof(p)]", "unsafe.Alignof(p)", 8},
	} {
		e := findExpr(t, prog, p, lineOf(t, src, c.at), c.at, c.src)
		got := p.Info.Types[e].Value
		if got == nil || !constant.Compare(got, token.EQL, constant.MakeInt64(c.want)) {
			t.Errorf("%s in an opaque argument is %v, want %d", c.src, got, c.want)
		}
	}
}

// TestOpaqueArgScopes checks that each argument is checked in its own
// scope: in a closure, in a function value of a package-level variable, in
// a method that uses its receiver's type parameter (in its result and its
// body), and in a generic function with two type parameters.
func TestOpaqueArgScopes(t *testing.T) {
	src := `package main

import (
	"<map>"
	"<vector>"
)

type Box[T any] struct{ items []T }

func (b *Box[T]) M() std.vector[T] { // result
	var v std.vector[T] // body
	return v
}

func Pair[T, U any](t T, u U) {
	var m std.map[U, std.vector[T]]
	_, _, _ = m, t, u
}

var h = func() {
	type L1 struct{}
	var v std.vector[L1]
	_ = v
}

func main() {
	f := func(x int) {
		type L2 struct{ n int }
		var v std.vector[L2]
		_, _ = v, x
	}
	f(1)
	h()
	Pair(1, "a")
	_ = (&Box[int]{}).M
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	scope := p.Types.Scope()
	m, _, _ := types.LookupFieldOrMethod(types.NewPointer(scope.Lookup("Box").Type()), false, p.Types, "M")
	recvT := m.(*types.Func).Signature().RecvTypeParams().At(0).Obj()
	pairSig := scope.Lookup("Pair").(*types.Func).Signature()
	localType := func(name string) types.Object {
		for id, obj := range p.Info.Defs {
			if id.Name == name {
				return obj
			}
		}
		t.Fatalf("no definition of %s", name)
		return nil
	}
	for _, c := range []struct {
		marker, at, src string
		want            types.Object
	}{
		{"// result", "T]", "T", recvT},
		{"// body", "T]", "T", recvT},
		{"std.map[U", "U,", "U", pairSig.TypeParams().At(1).Obj()},
		{"std.map[U", "T]", "T", pairSig.TypeParams().At(0).Obj()},
		{"vector[L1]", "L1]", "L1", localType("L1")},
		{"vector[L2]", "L2]", "L2", localType("L2")},
	} {
		e := findExpr(t, prog, p, lineOf(t, src, c.marker), c.at, c.src)
		if got := p.Info.Uses[e.(*ast.Ident)]; got != c.want {
			t.Errorf("%s at %q is %v, want %v", c.src, c.marker, got, c.want)
		}
		if tv, ok := p.Info.Types[e]; !ok || !tv.IsType() {
			t.Errorf("%s at %q has no type", c.src, c.marker)
		}
	}
}

// TestOpaqueArgMultiLine checks the places of errors in arguments that
// span several lines.
func TestOpaqueArgMultiLine(t *testing.T) {
	src := `package main

import (
	"<array>"
	"<vector>"
)

var x std.array[int,
	len(
		Pointt,
	),
]

var y = std.vector[
	Pointtt,
]{}

func main() { _, _ = x, y }
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:10:3: undefined: Pointt\n" +
		"./main.gp:15:2: undefined: Pointtt\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
}

// TestTemplateArgErrors checks the template arguments that C++ can't take
// (badTemplateArg): a generic type without type arguments, and a value
// that is not a constant (also a function variable or a method value), in
// a type and in a value. Constants, instantiated types, values with C/C++
// parts, functions and nil are fine.
func TestTemplateArgErrors(t *testing.T) {
	const box = "type Box[T any] struct{ v T }\n"
	const generic = "cannot use generic type Box[T any] without instantiation"
	for _, c := range []struct{ body, want string }{
		{box + "_ = std.vector[Box]{}", generic},
		{box + "var v std.vector[Box]\n_ = v", generic},
		{box + "_ = std.make_unique[Box]()", generic},
		{box + "_ = std.map[int, std.vector[Box]]{}", generic},
		{"xs := []int{1}\nvar a std.array[int, len(xs)]\n_ = a",
			"template argument len(xs) (value of type int) must be constant"},
		{"n := 3\nvar a std.array[int, n]\n_ = a",
			"template argument n (variable of type int) must be constant"},
		{"n := 3\n_ = std.get[n](lib.CB)",
			"template argument n (variable of type int) must be constant"},
		{box + "_ = std.vector[Box[int]]{}", ""},
		{"const k = 3\nvar a std.array[int, k*2]\n_ = a", ""},
		{"var arr [4]int\nvar a std.array[int, len(arr)]\n_ = a", ""},
		{"var a std.array[int, x.BUFSIZ]\n_ = a", ""},
		{"var a std.array[int, int(x.BUFSIZ)]\n_ = a", ""},
		{"_ = std.thing[main]{}", ""},
		{"_ = std.thing[lib.Make]{}", ""},
		{"_ = std.thing[nil]{}", ""},
		{"f := func() {}\n_ = std.thing[f]{}",
			"template argument f (variable of type func()) must be constant"},
		{"var p P\n_ = std.thing[p.Len]{}",
			"template argument p.Len (value of type func() float64) must be constant"},
	} {
		if got := falseErrorMessages(t, c.body); got != c.want {
			t.Errorf("body:\n%s\nerrors:\n%s\nwant:\n%s", c.body, got, c.want)
		}
	}
}
