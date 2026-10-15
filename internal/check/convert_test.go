package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// localType returns the type of the variable named name that the root
// package declares (the names in a test are unique).
func localType(t *testing.T, p *Package, name string) types.Type {
	t.Helper()
	for id, obj := range p.Info.Defs {
		if id.Name == name && obj != nil {
			return obj.Type()
		}
	}
	t.Fatalf("no declaration of %s", name)
	return nil
}

// conversionList returns the conversions of p as "line:col T(x)" lines,
// with the call as the user wrote it.
func conversionList(fset *token.FileSet, p *Package) string {
	var b strings.Builder
	for _, c := range p.Conversions {
		pos := fset.Position(c.Pos())
		b.WriteString(strings.TrimPrefix(pos.String(), pos.Filename+":"))
		b.WriteString(" " + c.String())
		switch c.Kind {
		case UnusedStmt:
			b.WriteString(" (unused)")
		case Deferred:
			b.WriteString(" (deferred)")
		case Compare:
			b.WriteString(" (compare)")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// TestConversions checks that a conversion of an opaque value has the
// type of the conversion, so that later mistakes are reported: of a call
// of a header package, of a variable from one, of C.sqrt, to []byte and to
// a parenthesized pointer type, in a call argument and in a return. The
// messages show the conversions as the user wrote them, also for a
// conversion that is a statement (an error in Go), and the rewritten ASTs
// keep their positions in order. A conversion after defer or go gets Go's
// error too ("defer requires function call, not conversion").
func TestConversions(t *testing.T) {
	src := `package main

import (
	"fmt"

	cstring "<string.h>"
)

type Point struct{ X, Y int }

func length(p *C.char) int {
	return int(cstring.strlen(p))
}

func main() {
	p := C.CString("hi")
	n := int(cstring.strlen(p))
	x := cstring.strlen(p)
	m := int(x)
	f := float64(C.sqrt(2))
	b := []byte(C.GoString(p))
	pt := (*Point)(C.malloc(16))
	fmt.Println(int(cstring.strlen(p)), m, f, b, pt.X, length(p))
	_ = n + "x"
	_ = m.size
	_ = pt.Z
	int(C.y())
	(float64(C.y()))
	var s string = int(C.f())
	_ = s
	defer int(C.f())
	go float64(C.f())
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:24:6: invalid operation: n + \"x\" (mismatched types int and untyped string)\n" +
		"./main.gp:25:8: m.size undefined (type int has no field or method size)\n" +
		"./main.gp:26:9: pt.Z undefined (type *Point has no field or method Z)\n" +
		"./main.gp:27:2: int(C.y()) (value of type int) is not used\n" +
		"./main.gp:28:2: (float64(C.y())) (value of type float64) is not used\n" +
		"./main.gp:29:17: cannot use int(C.f()) (value of type int) as string value in variable declaration\n" +
		"./main.gp:31:8: defer requires function call, not conversion int(C.f()) (value of type int)\n" +
		"./main.gp:32:5: go requires function call, not conversion float64(C.f()) (value of type float64)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	for name, typ := range map[string]string{
		"n": "int", "m": "int", "f": "float64", "b": "[]byte", "pt": "*example.com/m.Point",
		"x": "invalid type", "s": "string",
	} {
		if got := localType(t, p, name).String(); got != typ {
			t.Errorf("%s has the type %s, want %s", name, got, typ)
		}
	}
	wantConv := "12:9 int(cstring.strlen(p))\n" +
		"17:7 int(cstring.strlen(p))\n" +
		"19:7 int(x)\n" +
		"20:7 float64(C.sqrt(2))\n" +
		"21:7 []byte(C.GoString(p))\n" +
		"22:8 (*Point)(C.malloc(16))\n" +
		"23:14 int(cstring.strlen(p))\n" +
		"27:2 int(C.y()) (unused)\n" +
		"28:3 float64(C.y()) (unused)\n" +
		"29:17 int(C.f())\n" +
		"31:8 int(C.f()) (deferred)\n" +
		"32:5 float64(C.f()) (deferred)\n"
	// types.ExprString prints the rewritten C.strlen, not cstring.strlen.
	wantConv = strings.ReplaceAll(wantConv, "cstring.strlen", "C.strlen")
	if got := conversionList(prog.Fset, p); got != wantConv {
		t.Errorf("conversions:\n%s\nwant:\n%s", got, wantConv)
	}
	if p.checks != 2 {
		t.Errorf("checked %d times, want 2", p.checks)
	}

	for _, c := range p.Conversions {
		if c.Kind == Deferred {
			// The Value call covers the text of x.
			inner := c.Call.Args[0].(*ast.CallExpr)
			if c.Call.Fun != c.Type || inner.Pos() != c.Arg().Pos() || inner.End() != c.Arg().End() {
				t.Errorf("%s: the Value call is at %d-%d, x at %d-%d",
					c, inner.Pos(), inner.End(), c.Arg().Pos(), c.Arg().End())
			}
		} else {
			ix := c.Call.Fun.(*ast.IndexExpr)
			if ix.Index != c.Type || ix.Pos() != c.Type.Pos() || ix.End() != c.Type.End() {
				t.Errorf("%s: the instantiation is at %d-%d, T at %d-%d",
					c, ix.Pos(), ix.End(), c.Type.Pos(), c.Type.End())
			}
		}
		if p.Conversion(c.Call) != c {
			t.Errorf("%s: Conversion doesn't find it", c)
		}
		want := p.Info.Types[c.Type].Type
		if c.Kind == UnusedStmt {
			want = types.NewArray(want, 1)
		}
		if got := p.Info.Types[c.Call].Type; !types.Identical(got, want) {
			t.Errorf("%s has the type %v, want %v", types.ExprString(c.Original()), got, want)
		}
	}

	// The import is at the import keyword, and the declarations stay in
	// source order.
	f := prog.Roots[0].Files[0].AST
	spec := cgohImport(f)
	if spec == nil || spec.Name.Name != "gp_cgoh" {
		t.Fatalf("the import of gocpp/cgoh is %v", spec)
	}
	gd := f.Decls[0].(*ast.GenDecl)
	if spec.Pos() != gd.TokPos || gd.Specs[len(gd.Specs)-1] != spec || f.Imports[len(f.Imports)-1] != spec {
		t.Errorf("the import is at %v, the import keyword at %v", prog.Fset.Position(spec.Pos()), prog.Fset.Position(gd.TokPos))
	}
	for i := 1; i < len(f.Decls); i++ {
		if f.Decls[i].Pos() < f.Decls[i-1].End() {
			t.Errorf("declaration %d starts before the end of declaration %d", i, i-1)
		}
	}
}

// TestConversionCTypes checks conversions to types with C parts: []C.int
// and a struct or a named pointer type with a C part are rewritten (the
// result has the type, with an invalid part, like a variable declared
// with it), but *C.FILE has no type in go/types, so (*C.FILE)(x) stays.
// Conversions to interfaces, and in a range clause and a type switch
// guard, are rewritten too.
func TestConversionCTypes(t *testing.T) {
	src := `package main

type P2 struct {
	x C.int
	y int
}

type H *C.FILE

func main() {
	a := []C.int(C.f())
	b := (*C.FILE)(C.f())
	c := P2(C.g())
	d := H(C.h())
	_ = c.z
	_ = c.y + "a"
	_ = a[0] + "a"
	f := any(C.f())
	g := error(C.f())
	_, _, _, _, _ = a, b, d, f, g
	for _, v := range []int(C.v()) {
		_ = v + "a"
	}
	switch w := any(C.f()).(type) {
	case int:
		_ = w + "a"
	}
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:15:8: c.z undefined (type P2 has no field or method z)\n" +
		"./main.gp:16:6: invalid operation: c.y + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:22:7: invalid operation: v + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:26:7: invalid operation: w + \"a\" (mismatched types int and untyped string)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	wantConv := "11:7 []C.int(C.f())\n" +
		"13:7 P2(C.g())\n" +
		"14:7 H(C.h())\n" +
		"18:7 any(C.f())\n" +
		"19:7 error(C.f())\n" +
		"21:20 []int(C.v())\n" +
		"24:14 any(C.f())\n"
	if got := conversionList(prog.Fset, p); got != wantConv {
		t.Errorf("conversions:\n%s\nwant:\n%s", got, wantConv)
	}
	for name, typ := range map[string]string{
		"a": "[]invalid type", "b": "invalid type", "c": "example.com/m.P2", "d": "example.com/m.H",
		"f": "any", "g": "error",
	} {
		if got := localType(t, p, name).String(); got != typ {
			t.Errorf("%s has the type %s, want %s", name, got, typ)
		}
	}
}

// TestConversionTyped checks that a conversion of a typed value is not
// rewritten, also in a file with C.
func TestConversionTyped(t *testing.T) {
	src := `package main

func main() {
	var v float64
	n := int(v)
	s := string(rune(n))
	q := (*int)(nil)
	_, _ = s, q
	_ = C.f()
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	if len(p.Conversions) != 0 || p.checks != 1 {
		t.Errorf("%d conversions, %d checks, want 0 and 1", len(p.Conversions), p.checks)
	}
	if cgohImport(prog.Roots[0].Files[0].AST) != nil {
		t.Errorf("the file imports gocpp/cgoh")
	}
}

// TestConversionNoC checks a package without C: nothing is rewritten, and
// it is checked once. A conversion of a value that an error made invalid
// gives no more errors, like in go build.
func TestConversionNoC(t *testing.T) {
	src := `package main

func main() {
	n := int(undefinedName)
	_ = n + "a"
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n./main.gp:4:11: undefined: undefinedName\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	if p.Conversions != nil || p.checks != 1 {
		t.Errorf("%d conversions, %d checks, want none and 1", len(p.Conversions), p.checks)
	}
}

// TestConversionErrors checks the conversions that are not rewritten,
// because an error can be the reason that their value is invalid: a
// variable from an undefined name, a parameter of an undefined type, a
// field of a struct with an undefined field type, an element of a named
// type with an undefined element. go build reports only the errors,
// nothing about n1, n3, n4 and n5. An error in the arguments of a C call
// doesn't count: C.f(undefinedQ) is opaque whatever its argument is, so n2
// is an int, like in a Go twin where f returns an int (go build reports
// undefinedQ and n2 + "a").
func TestConversionErrors(t *testing.T) {
	src := `package main

type S struct{ f Undef2 }

type T []Undef3

func g(p Undef1) int {
	n3 := int(p)
	_ = n3 + "a"
	return 0
}

func main() {
	x := undefinedName
	n1 := int(x)
	_ = n1 + "a"
	n2 := int(C.f(undefinedQ))
	_ = n2 + "a"
	var s S
	n4 := int(s.f)
	_ = n4 + "a"
	var t T
	n5 := int(t[0])
	_ = n5 + "a"
	ok := int(C.f(t))
	_ = ok + "a"
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	// ok is opaque: C.f(t) is invalid whatever t is.
	want := "# example.com/m\n" +
		"./main.gp:3:18: undefined: Undef2\n" +
		"./main.gp:5:10: undefined: Undef3\n" +
		"./main.gp:7:10: undefined: Undef1\n" +
		"./main.gp:14:7: undefined: undefinedName\n" +
		"./main.gp:17:16: undefined: undefinedQ\n" +
		"./main.gp:18:6: invalid operation: n2 + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:26:6: invalid operation: ok + \"a\" (mismatched types int and untyped string)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if got := conversionList(prog.Fset, mainPkg(t, prog, r)); got != "17:8 int(C.f(undefinedQ))\n25:8 int(C.f(t))\n" {
		t.Errorf("conversions:\n%s", got)
	}
}

// TestConversionErrorOrder checks the errors after the second check
// against a Go twin of the program, in which D.strlen is a Go function:
// the same errors in the same order, also with the limit of 10 errors
// (which keeps the first 10 found), and both equal to go build 1.25.14 on
// the twin. Some of the errors are there only because the conversions of
// C values are typed (n + "a", m.foo).
func TestConversionErrorOrder(t *testing.T) {
	src := `package main

func main() {
	var p *byte
	n := int(C.strlen(p))
	_ = n + "a"
	x := C.strlen(p)
	m := int(x)
	m.foo()
	var e1 int = "s"
	k := int(int(C.strlen(p)))
	_ = k + "b"
	f := float64(C.strlen(p))
	_ = f + "c"
	unused := int(C.strlen(p))
	var e2 string = 1
	_ = n.bar
	_ = len(n)
	var e3 bool = 2
	_ = m + "d"
	_ = !n
}

func g() {
	var e4 int = "g"
}

var late int = "late"
`
	twin := strings.ReplaceAll(src, "C.", "D.") + "\nvar D struct{ strlen func(*byte) uintptr }\n"
	// go build and go build -gcflags=-e of the twin as main.go (go1.25.14).
	wantLimit := `./main.gp:6:6: invalid operation: n + "a" (mismatched types int and untyped string)
./main.gp:9:4: m.foo undefined (type int has no field or method foo)
./main.gp:10:15: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:12:6: invalid operation: k + "b" (mismatched types int and untyped string)
./main.gp:14:6: invalid operation: f + "c" (mismatched types float64 and untyped string)
./main.gp:16:18: cannot use 1 (untyped int constant) as string value in variable declaration
./main.gp:17:8: n.bar undefined (type int has no field or method bar)
./main.gp:18:10: invalid argument: n (variable of type int) for built-in len
./main.gp:19:16: cannot use 2 (untyped int constant) as bool value in variable declaration
./main.gp:28:16: cannot use "late" (untyped string constant) as int value in variable declaration
./main.gp:19:16: too many errors
`
	wantAll := `./main.gp:6:6: invalid operation: n + "a" (mismatched types int and untyped string)
./main.gp:9:4: m.foo undefined (type int has no field or method foo)
./main.gp:10:6: declared and not used: e1
./main.gp:10:15: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:12:6: invalid operation: k + "b" (mismatched types int and untyped string)
./main.gp:14:6: invalid operation: f + "c" (mismatched types float64 and untyped string)
./main.gp:15:2: declared and not used: unused
./main.gp:16:6: declared and not used: e2
./main.gp:16:18: cannot use 1 (untyped int constant) as string value in variable declaration
./main.gp:17:8: n.bar undefined (type int has no field or method bar)
./main.gp:18:10: invalid argument: n (variable of type int) for built-in len
./main.gp:19:6: declared and not used: e3
./main.gp:19:16: cannot use 2 (untyped int constant) as bool value in variable declaration
./main.gp:20:6: invalid operation: m + "d" (mismatched types int and untyped string)
./main.gp:21:7: invalid operation: operator ! not defined on n (variable of type int)
./main.gp:25:6: declared and not used: e4
./main.gp:25:15: cannot use "g" (untyped string constant) as int value in variable declaration
./main.gp:28:16: cannot use "late" (untyped string constant) as int value in variable declaration
`
	for _, c := range []struct{ name, src string }{{"gocpp", src}, {"twin", twin}} {
		root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": c.src})
		prog, r := loadAndCheck(t, load.Config{}, root, ".")
		for _, all := range []bool{false, true} {
			want := "# example.com/m\n" + wantLimit
			if all {
				want = "# example.com/m\n" + wantAll
			}
			if errs := errorText(t, prog, r, root, all); errs != want {
				t.Errorf("%s, all %v:\n%s\nwant:\n%s", c.name, all, errs, want)
			}
		}
	}
}

// TestConversionConstants checks that conversions where Go needs a
// constant are not rewritten (As[T](x) is not a constant): in a const
// declaration, an array length and a key of a slice literal. The key of a
// map literal needs no constant.
func TestConversionConstants(t *testing.T) {
	src := `package main

const c = int(C.N)

const (
	d = int(C.N) + 1
	e
)

var a [int(C.N)]int

func main() {
	s := []int{int(C.K): 1}
	m := map[int]int{int(C.K): 1}
	_, _, _, _, _ = a, s, m, d, e
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	if got := conversionList(prog.Fset, mainPkg(t, prog, r)); got != "14:19 int(C.K)\n" {
		t.Errorf("conversions:\n%s", got)
	}
}

// TestConversionRounds checks conversions whose argument depends on
// another conversion: they wait for the next check, and then they are Go
// conversions of typed values (string(rune(x)) is a rune conversion, not a
// C++ cast). An argument that stays opaque after the other conversion
// (float64(C.a()) + C.b()) is rewritten at once. A conversion inside the
// arguments of a C call doesn't make the call wait.
func TestConversionRounds(t *testing.T) {
	src := `package main

func main() {
	k := int(int(C.f()))
	x := int(C.f())
	r := string(rune(x))
	y := float64(C.a()) + C.b()
	n := int(y)
	z := int(C.g(float64(C.h())))
	_, _, _, _ = k, r, n, z
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	want := "4:11 int(C.f())\n" +
		"5:7 int(C.f())\n" +
		"7:7 float64(C.a())\n" +
		"8:7 int(y)\n" +
		"9:7 int(C.g(float64(C.h())))\n" +
		"9:15 float64(C.h())\n"
	if got := conversionList(prog.Fset, p); got != want {
		t.Errorf("conversions:\n%s\nwant:\n%s", got, want)
	}
	if p.checks != 2 {
		t.Errorf("checked %d times, want 2", p.checks)
	}
	for name, typ := range map[string]string{"k": "int", "r": "string", "n": "int", "z": "int", "y": "invalid type"} {
		if got := localType(t, p, name).String(); got != typ {
			t.Errorf("%s has the type %s, want %s", name, got, typ)
		}
	}
}

// TestConversionNames checks that the import of gocpp/cgoh never clashes
// with a user's name: gp_cgoh (a package-level name of another file) and
// gp_cgoh_1 (a local) are taken, so the import is gp_cgoh_2. A file
// without imports gets a new import declaration at its package keyword,
// and a conversion of a C value of another file is rewritten too.
func TestConversionNames(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "package main\n\nvar gp_cgoh = C.f()\n",
		"b.gp": `package main

func main() {
	gp_cgoh_1 := 2
	n := int(gp_cgoh)
	_ = n + gp_cgoh_1
	_ = n + "a"
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n./b.gp:7:6: invalid operation: n + \"a\" (mismatched types int and untyped string)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	f := prog.Roots[0].Files[1].AST
	spec := cgohImport(f)
	if spec == nil || spec.Name.Name != "gp_cgoh_2" || spec.Pos() != f.Package {
		t.Fatalf("the import of gocpp/cgoh is %v at %v", spec, spec.Pos())
	}
	if cgohImport(prog.Roots[0].Files[0].AST) != nil {
		t.Errorf("a.gp imports gocpp/cgoh")
	}
	if got := localType(t, mainPkg(t, prog, r), "n").String(); got != "int" {
		t.Errorf("n has the type %s", got)
	}
}

// TestConversionVersion checks a file with an old version (go1.17, before
// generics): go/types checks it as go1.21, so the rewrite works there too.
func TestConversionVersion(t *testing.T) {
	src := `//go:build go1.17

package main

func main() {
	n := int(C.f())
	_ = n + "a"
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n./main.gp:7:6: invalid operation: n + \"a\" (mismatched types int and untyped string)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Fatalf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if v := mainPkg(t, prog, r).Info.FileVersions[prog.Roots[0].Files[0].AST]; v != "go1.21" {
		t.Errorf("the file version is %q", v)
	}
}

// TestConversionBrokenImport checks that a package that imports a package
// with errors gets no rewrite: an invalid value from there can come from
// an error (go build doesn't check such a package at all). go/types
// reports the conversion of a.Y, the first error of main, as it does
// without the rewrite.
func TestConversionBrokenImport(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nvar X int = \"s\"\n\nvar Y = undefinedY\n",
		"main.gp": `package main

import "example.com/m/a"

func main() {
	n := int(C.f())
	_ = n + "a"
	k := int(a.Y)
	_ = k + "a"
	_ = a.X
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m/a\n" +
		"a/a.gp:3:13: cannot use \"s\" (untyped string constant) as int value in variable declaration\n" +
		"a/a.gp:5:9: undefined: undefinedY\n" +
		"# example.com/m\n" +
		"./main.gp:8:11: cannot convert a.Y (variable with invalid type) to type int\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if p := mainPkg(t, prog, r); len(p.Conversions) != 0 || p.checks != 1 {
		t.Errorf("%d conversions, %d checks", len(p.Conversions), p.checks)
	}
}

// TestConversionOtherPackage checks opaque values of another package, in
// a package without C: a variable, the result of a function with a C
// result type, and a dereference of it. Before the rewrite, go/types
// reported "cannot convert a.X (variable with invalid type) to type int".
func TestConversionOtherPackage(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nvar X = C.f()\n\nfunc F() *C.FILE { return nil }\n",
		"b/b.gp":    "package b\n\nimport \"example.com/m/a\"\n\nvar Y = a.X\n",
		"main.gp": `package main

import (
	"example.com/m/a"
	"example.com/m/b"
)

func main() {
	n := int(a.X)
	_ = n + "a"
	k := int(*a.F())
	_ = k + "a"
	p := uintptr(a.F())
	_ = p + "a"
	y := int(b.Y)
	_ = y + "a"
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:10:6: invalid operation: n + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:12:6: invalid operation: k + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:14:6: invalid operation: p + \"a\" (mismatched types uintptr and untyped string)\n" +
		"./main.gp:16:6: invalid operation: y + \"a\" (mismatched types int and untyped string)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	for _, p := range r.Packages {
		want := 0
		if p.Load == prog.Roots[0] {
			want = 4
		}
		if len(p.Conversions) != want {
			t.Errorf("%s: %d conversions, want %d", p.Load.ImportPath, len(p.Conversions), want)
		}
	}
}

// TestConversionOpaqueArg checks a conversion in a type argument of an
// opaque instantiation, which the CheckExpr pass checks: it is rewritten,
// and the import of gocpp/cgoh counts as used.
func TestConversionOpaqueArg(t *testing.T) {
	src := `package main

import "<array>"

var a std.array[int, int(C.N)]

func main() { _ = a }
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	if got := conversionList(prog.Fset, mainPkg(t, prog, r)); got != "5:22 int(C.N)\n" {
		t.Errorf("conversions:\n%s", got)
	}
}

// TestConversionCheckTwice checks the same loaded program twice: the ASTs
// are rewritten already, so the second Check needs one check per package,
// and it finds the same conversions and errors.
func TestConversionCheckTwice(t *testing.T) {
	src := `package main

func main() {
	n := int(C.f())
	_ = n + "a"
	int(C.g())
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	r2 := Check(prog, fakeProbeConfig(LastPhase))
	errs, errs2 := errorText(t, prog, r, root, true), errorText(t, prog, r2, root, true)
	want := "# example.com/m\n" +
		"./main.gp:5:6: invalid operation: n + \"a\" (mismatched types int and untyped string)\n" +
		"./main.gp:6:2: int(C.g()) (value of type int) is not used\n"
	if errs != want || errs2 != want {
		t.Errorf("errors:\n%s\nthe second time:\n%s\nwant:\n%s", errs, errs2, want)
	}
	p, p2 := mainPkg(t, prog, r), mainPkg(t, prog, r2)
	if c, c2 := conversionList(prog.Fset, p), conversionList(prog.Fset, p2); c != c2 || len(p2.Conversions) != 2 {
		t.Errorf("conversions:\n%s\nthe second time:\n%s", c, c2)
	}
	if p.checks != 2 || p2.checks != 1 {
		t.Errorf("checked %d and %d times, want 2 and 1", p.checks, p2.checks)
	}
}

// TestUnwrap checks how the messages show a rewritten conversion.
func TestUnwrap(t *testing.T) {
	cv := &converter{names: map[string]bool{"gp_cgoh": true}}
	for _, c := range []struct{ in, want string }{
		{"gp_cgoh.As[int](C.f()) (value of type int)", "int(C.f()) (value of type int)"},
		{"cannot use gp_cgoh.As[[]byte](gp_cgoh.As[string](x)) as T", "cannot use []byte(string(x)) as T"},
		{"(gp_cgoh.As[(*T)](p)).X", "((*T)(p)).X"},
		{"gp_cgoh.As[map[string]int](C.m()) + 1", "map[string]int(C.m()) + 1"},
		{"gp_cgoh.Unused[int](C.y())[0] (value of type int) is not used", "int(C.y()) (value of type int) is not used"},
		{"gp_cgoh.Unused[int](gp_cgoh.As[float64](C.y()))[0] is not used", "int(float64(C.y())) is not used"},
		{"xgp_cgoh.As[int](y) and gp_cgoh.As[int(", "xgp_cgoh.As[int](y) and gp_cgoh.As[int("},
		{"assignment mismatch: 2 variables but gp_cgoh.As[int] returns 1 value", "assignment mismatch: 2 variables but int returns 1 value"},
		{"gp_cgoh.As[[]byte]", "[]byte"},
		{"defer requires function call, not conversion int(gp_cgoh.Value[int](C.f())) (value of type int)",
			"defer requires function call, not conversion int(C.f()) (value of type int)"},
		{"go float64(gp_cgoh.Value[float64](gp_cgoh.As[int](x)))", "go float64(int(x))"},
		{"gp_cgoh.As[struct{a int \"json:\\\"]\\\"\"; b int `x)`}](x).a", "struct{a int \"json:\\\"]\\\"\"; b int `x)`}(x).a"},
		{"gp_cgoh.Unused[int](x) stays", "gp_cgoh.Unused[int](x) stays"},
	} {
		if got := cv.fixMessage(c.in); got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
		}
	}
}

// twinErrors checks the program src (one file, main.gp) and its Go twin,
// in which C.f is D.f, a Go function that returns an int (D is declared
// after src, so the places are the same; C.int becomes D.int, an int
// field), and returns their errors, the twin's with D. shown as C. again.
func twinErrors(t *testing.T, src string) (gocpp, twin string) {
	t.Helper()
	run := func(src string) string {
		root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
		prog, r := loadAndCheck(t, load.Config{}, root, ".")
		return errorText(t, prog, r, root, true)
	}
	twinSrc := strings.ReplaceAll(src, "C.", "D.") + "\nvar D struct{ f, g func(...any) int }\n"
	twinSrc = strings.ReplaceAll(twinSrc, "D.int", "int  ")
	return run(src), strings.ReplaceAll(run(twinSrc), "D.", "C.")
}

// TestConversionTwins checks programs against their Go twins (see
// twinErrors), which the test checks with Check too (pure Go, so with
// go/types as go build): the same errors. The want strings come from one
// run of real go build 1.25.14 -gcflags=-e on the twins. In the first programs, a value is invalid for a reason
// that the analysis doesn't follow, or because of an error at the name of
// a declaration: such a conversion is not rewritten, also with a C source
// next to it, so that it gives no errors that go build doesn't give. In
// the others, the conversion is rewritten: an error in the arguments of a
// C call, a conversion with two variables on the left, and after go and
// defer.
func TestConversionTwins(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"recursive type", `package main

type T struct{ a T }

func main() {
	var v T
	n := int(v.a + C.f())
	_ = n + "a"
}
`, `./main.gp:3:6: invalid recursive type: T refers to itself
`},
		{"initialization cycle", `package main

var a = b + C.f()
var b = a

func main() {
	n := int(a)
	_ = n + "x"
}
`, `./main.gp:3:5: initialization cycle for a
	./main.gp:3:5: a refers to b
	./main.gp:4:5: b refers to a
`},
		{"range over int", `package main

func main() {
	for i, v := range 10 {
		n := int(C.f() + v)
		_ = n + "a"
		_ = i
	}
}
`, `./main.gp:4:9: range over 10 (untyped int constant) permits only one iteration variable
`},
		{"range over float64", `package main

var f float64

func main() {
	for _, v := range f {
		n := int(C.f() + v)
		_ = n + "a"
	}
}
`, `./main.gp:6:20: cannot range over f (variable of type float64)
`},
		{"error in a C argument", `package main

func main() {
	n := int(C.f(undefinedQ))
	_ = n + "a"
}
`, `./main.gp:4:15: undefined: undefinedQ
./main.gp:5:6: invalid operation: n + "a" (mismatched types int and untyped string)
`},
		{"assignment mismatch", `package main

func main() {
	a, b := int(C.f())
	var c, d = int(C.f())
	var e, g int
	e, g = int(C.f())
	_, _, _, _, _, _ = a, b, c, d, e, g
}
`, `./main.gp:4:10: assignment mismatch: 2 variables but int returns 1 value
./main.gp:5:13: assignment mismatch: 2 variables but int returns 1 value
./main.gp:7:9: assignment mismatch: 2 variables but int returns 1 value
`},
		{"field next to an error", `package main

type S struct {
	a Undef
	b C.int
}

func main() {
	var s S
	n := int(s.b)
	_ = n + "a"
}
`, `./main.gp:4:4: undefined: Undef
./main.gp:11:6: invalid operation: n + "a" (mismatched types int and untyped string)
`},
		{"go and defer", `package main

func main() {
	x := 1
	_ = x + "a"
	defer int(C.f())
	go float64(C.g(x))
}
`, `./main.gp:5:6: invalid operation: x + "a" (mismatched types int and untyped string)
./main.gp:6:8: defer requires function call, not conversion int(C.f()) (value of type int)
./main.gp:7:5: go requires function call, not conversion float64(C.g(x)) (value of type float64)
`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, twin := twinErrors(t, c.src)
			want := "# example.com/m\n" + c.want
			if got != want || twin != want {
				t.Errorf("errors:\n%s\nthe twin:\n%s\nwant:\n%s", got, twin, want)
			}
		})
	}
}

// TestConversionWaits checks why a conversion of a conversion to a type
// with a C part waits for the next check (see converter): int(P2(C.f())) is
// a Go error once P2(C.f()) has its type, directly and through a variable,
// as in a Go twin where f returns a P2 (the want strings are go build's).
func TestConversionWaits(t *testing.T) {
	src := `package main

type P2 struct {
	x C.int
	y int
}

func main() {
	n := int(P2(C.f()))
	p := P2(C.f())
	m := int(p)
	_, _ = n, m
}
`
	run := func(src string) string {
		root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
		prog, r := loadAndCheck(t, load.Config{}, root, ".")
		return errorText(t, prog, r, root, true)
	}
	twin := strings.ReplaceAll(strings.ReplaceAll(src, "C.int", "int  "), "C.f", "D.f") +
		"\nvar D struct{ f func(...any) P2 }\n"
	want := "# example.com/m\n" +
		"./main.gp:9:11: cannot convert P2(%s.f()) (value of struct type P2) to type int\n" +
		"./main.gp:11:11: cannot convert p (variable of struct type P2) to type int\n"
	if got, w := run(src), fmt.Sprintf(want, "C"); got != w {
		t.Errorf("errors:\n%s\nwant:\n%s", got, w)
	}
	if got, w := run(twin), fmt.Sprintf(want, "D"); got != w {
		t.Errorf("the twin:\n%s\nwant:\n%s", got, w)
	}
}

// TestConversionGates checks that the phase gates see the conversions as
// the user wrote them, T(x): not the import of gocpp/cgoh, the generic
// function As (generics, Phase 07), the [1]T of a statement or the copy of
// T after defer. At Phase 04, which has C, the errors are go/types' only.
// At Phase 11, a member of a C++ type promoted through an embedded field
// (Phase 12) is gated also on a conversion, W(C.f()).fileno: the gate
// matches go/types' message, which shows the conversion as the user wrote
// it.
func TestConversionGates(t *testing.T) {
	src := `package main

import cstring "<string.h>"

func main() {
	p := C.CString("hi")
	n := int(cstring.strlen(p))
	_ = n
	int(C.y())
	defer int(C.f())
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog := load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	r := Check(prog, fakeProbeConfig(4))
	want := "# example.com/m\n" +
		"./main.gp:9:2: int(C.y()) (value of type int) is not used\n" +
		"./main.gp:10:8: defer requires function call, not conversion int(C.f()) (value of type int)\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if n := len(mainPkg(t, prog, r).Conversions); n != 3 {
		t.Errorf("%d conversions, want 3", n)
	}

	src = `package main

type W struct{ *C.FILE }

func main() {
	var w W
	_ = w.fileno
	_ = W(C.f()).fileno
	_ = (*W)(C.p()).fileno
}
`
	root = writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog = load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	r = Check(prog, fakeProbeConfig(11))
	msg := features[featPromotedCpp].Message()
	want = "# example.com/m\n" +
		"./main.gp:7:8: " + msg + "\n" +
		"./main.gp:8:15: " + msg + "\n" +
		"./main.gp:9:18: " + msg + "\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if n := len(mainPkg(t, prog, r).Conversions); n != 2 {
		t.Errorf("%d conversions, want 2", n)
	}
}

// TestConversionValid checks valid programs with conversions of values
// that come from C through literals, make, maps, channels, function
// literals, a conversion to a type with a C part (three checks: see
// converter), and a struct that refers to itself: 0 errors, and n has the
// type int.
func TestConversionValid(t *testing.T) {
	for _, c := range []struct {
		name, decls, body string
		checks            int
	}{
		{"slice literal", "", "x := []C.int{1}\n\tn := int(x[0])", 2},
		{"array literal", "", "x := [2]C.float{1, 2}\n\tn := int(x[1])", 2},
		{"make", "", "x := make([]C.int, 3)\n\tn := int(x[0])", 2},
		{"map", "", "xs := map[string]C.int{}\n\tn := int(xs[\"a\"])", 2},
		{"channel", "", "ch := make(chan C.int)\n\tn := int(<-ch)", 2},
		{"function literal", "", "f := func() C.int { return 0 }\n\tn := int(f())", 2},
		{"conversion to []C.int", "", "a := []C.int(C.f())\n\tn := int(a[0])", 3},
		{"recursive C struct", "type L struct {\n\tnext *L\n\tv    C.int\n}\n",
			"var l L\n\tn := int(l.v) + int(l.next.v)", 2},
		{"embedded C field", "type E struct{ *C.FILE }\n",
			"var e E\n\tn := int(uintptr(e.FILE))", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := "package main\n\n" + c.decls + "\nfunc main() {\n\t" + c.body + "\n\t_ = n\n}\n"
			root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
			prog, r := loadAndCheck(t, load.Config{}, root, ".")
			if errs := errorText(t, prog, r, root, true); errs != "" {
				t.Errorf("errors:\n%s\nsource:\n%s", errs, src)
			}
			p := mainPkg(t, prog, r)
			if got := localType(t, p, "n").String(); got != "int" {
				t.Errorf("n has the type %s", got)
			}
			if p.checks != c.checks {
				t.Errorf("checked %d times, want %d", p.checks, c.checks)
			}
		})
	}
}

// TestComparisons checks the rewrite of comparisons of opaque values to
// gp_cgoh.Cmp(x == y) (Compare): they are bools, as cgo's comparisons of C
// values, so a variable declared from one is a bool, later mistakes with it
// are reported, the messages show the comparison as the user wrote it, a
// conversion of a comparison is a Go conversion, and a comparison where Go
// needs a constant, or of values that an error made invalid, stays.
func TestComparisons(t *testing.T) {
	src := `package main

import (
	"<lib.h>"
)

func isNil(p *C.char) bool { return p == nil }

func main() {
	var p *C.char
	b := p == nil
	c := lib.f() < 3 && !(lib.g() != lib.h())
	d := bool(lib.f() >= 2)
	_ = isNil(p) || b || c || d
	_ = b + 1
	var n int = lib.f() == 1
	_ = n
	var arr [lib.N]int
	_ = arr
	e := undefinedName == lib.f()
	_ = e
}
`
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:15:6: invalid operation: b + 1 (mismatched types bool and untyped int)\n" +
		"./main.gp:16:14: cannot use lib.f() == 1 (value of type bool) as int value in variable declaration\n" +
		"./main.gp:20:7: undefined: undefinedName\n"
	got := errorText(t, prog, r, root, true)
	got = strings.ReplaceAll(got, "C.f()", "lib.f()")
	if got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	p := mainPkg(t, prog, r)
	for name, typ := range map[string]string{"b": "bool", "c": "bool", "d": "bool", "e": "invalid type"} {
		if got := localType(t, p, name).String(); got != typ {
			t.Errorf("%s has the type %s, want %s", name, got, typ)
		}
	}
	wantConv := "7:37 p == nil (compare)\n" +
		"11:7 p == nil (compare)\n" +
		"12:7 C.f() < 3 (compare)\n" +
		"12:24 C.g() != C.h() (compare)\n" +
		"13:12 C.f() >= 2 (compare)\n" +
		"16:14 C.f() == 1 (compare)\n"
	if got := conversionList(prog.Fset, p); got != wantConv {
		t.Errorf("conversions:\n%s\nwant:\n%s", got, wantConv)
	}
	for _, c := range p.Conversions {
		if got := p.Info.Types[c.Call].Type; got != types.Typ[types.Bool] {
			t.Errorf("%s has the type %v, want bool", c, got)
		}
		if arg := c.Call.Args[0]; c.Call.Pos() != arg.Pos() || c.Call.End() != arg.End() {
			t.Errorf("%s: the call is at %d-%d, the comparison at %d-%d", c, c.Call.Pos(), c.Call.End(), arg.Pos(), arg.End())
		}
	}
}
