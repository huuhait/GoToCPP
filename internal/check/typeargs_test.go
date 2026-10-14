package check

import (
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

// TestOpaqueArgSizes checks that unsafe.Sizeof in an opaque argument gives
// the package's own size (gocppSizes). CheckExpr takes no types.Config and
// uses go/types' default sizes (gc on amd64), which are the same as the
// stub today. When Phase 03 gives gocppSizes the real C++ layout (a string
// is 32 bytes), this test fails, on purpose: the arguments must then get
// the same sizes another way.
func TestOpaqueArgSizes(t *testing.T) {
	src := `package main

import (
	"unsafe"

	"<array>"
)

type pair struct {
	a int8
	b string
}

var s string
var p pair

const (
	sizeS = unsafe.Sizeof(s)
	sizeP = unsafe.Sizeof(p)
)

var a std.array[byte, unsafe.Sizeof(s)]
var b std.array[byte, unsafe.Sizeof(p)]

func main() { _, _ = a, b }
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	for _, c := range []struct{ name, at, src string }{
		{"sizeS", "unsafe.Sizeof(s)]", "unsafe.Sizeof(s)"},
		{"sizeP", "unsafe.Sizeof(p)]", "unsafe.Sizeof(p)"},
	} {
		want := p.Types.Scope().Lookup(c.name).(*types.Const).Val()
		e := findExpr(t, prog, p, lineOf(t, src, c.at), c.at, c.src)
		got := p.Info.Types[e].Value
		if got == nil || !constant.Compare(got, token.EQL, want) {
			t.Errorf("%s in an opaque argument is %v, want %v (%s)", c.src, got, want, c.name)
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
