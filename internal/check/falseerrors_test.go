package check

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// falseErrorsMain is main.gp of the tests of the false errors: BODY is
// the body of main. Handle, V, H and P are named types over C/C++ types,
// GoInt and GoS their Go twins; package lib has a variable CB whose type
// is not complete.
const falseErrorsMain = `package main

import (
	"fmt"
	"unsafe"

	"example.com/m/lib"
	"<vector>"
	"<x.h>"
)

var (
	_ = fmt.Println
	_ = lib.CB
	_ unsafe.Pointer
)

type Handle C.int
type V C.vec
type H C.handle
type P C.Point

type GoInt int
type GoS struct{ x int }
type S struct{ n int }

func (p P) Len() float64 { return std.sqrt(p.x*p.x + p.y*p.y) }

func main() {
BODY
}
`

// falseErrorsLib is lib/lib.gp of the tests of the false errors.
const falseErrorsLib = `package lib

import "<x.h>"

var CB = x.get_cb()

func Make() *x.Vec { return nil }

type Handle C.int
`

// falseErrorMessages checks main with the body and returns the messages of
// its errors and warnings, one per line. It also checks the scope tree,
// which CheckExpr on a skipped expression could break.
func falseErrorMessages(t *testing.T, body string) string {
	t.Helper()
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    strings.Replace(falseErrorsMain, "BODY", body, 1),
		"lib/lib.gp": falseErrorsLib,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if prog.Errors.HasErrors() {
		t.Fatalf("load errors: %s", errorText(t, prog, r, root, true))
	}
	var msgs []string
	for _, p := range r.Packages {
		for _, d := range p.Errors.Diagnostics() {
			msgs = append(msgs, d.Msg)
		}
		if p.Checked() {
			checkScopeTree(t, p)
		}
	}
	return strings.Join(msgs, "\n")
}

// TestFalseErrors checks that the false errors of go/types on opaque code
// are dropped (positive cases, with no errors left: also no internal error
// of the opaque pass, so every expression without a type is opaque), and
// that the same code on Go values is still reported (negative cases). The
// first cases are the ones of the TODO list (job 1.11 and the 1.10b/1.11
// reviews).
func TestFalseErrors(t *testing.T) {
	for _, c := range []struct {
		name, body string
		want       string // the messages, one per line; "" for a positive case
	}{
		// The TODO list.
		{"range opaque literal", "for _, x := range (std.vector[int]{1, 2}) {\n\t_ = x\n}", ""},
		{"elided opaque literal", "_ = std.vector[std.vector[int]]{{1, 2}}", ""},
		{"named C int plus const", "var h Handle\n_ = h + 1", ""},
		{"indirect other package", "_ = *lib.CB", ""},
		{"assert other package", "_, _ = lib.CB.(fmt.Stringer)", ""},
		{"field of pointer to opaque", "p := &C.struct_s{}\n_ = int(p.x)", ""},
		{"elided keyed literal", "_ = std.vector[S]{{n: 1}}", ""},
		{"make named C", "n := 3\n_ = make(V, n)", ""},
		{"assert named C", "var h H\n_, _ = h.(fmt.Stringer)", ""},

		// Their families: the same codes on other C values.
		{"range other package", "for _, v := range lib.CB {\n\t_ = v\n}", ""},
		{"range opaque result", "for i, x := range lib.Make() {\n\t_, _ = i, x\n}", ""},
		{"range named C", "var v V\nfor i, x := range v {\n\t_, _ = i, x\n}", ""},
		{"range one var", "for i := range (std.vector[int]{1, 2}) {\n\t_ = i\n}", ""},
		{"elided in map", "_ = std.map[int, std.vector[int]]{1: {2}}", ""},
		{"elided in named C", "_ = V{{1}}", ""},
		{"unary other package", "_ = -lib.CB\n_ = !lib.CB", ""},
		{"index other package", "n := 1\n_ = lib.CB[n]", ""},
		{"slice other package", "n := 1\n_ = lib.CB[1:n]", ""},
		{"call other package", "n := 1\n_ = lib.CB(n)\ngo lib.CB()", ""},
		{"unary named C", "var h Handle\n_ = -h\n_ = !h", ""},
		{"binary named C", "n := 1\nvar h Handle\n_ = h + h\n_ = h < h\n_ = h + n\n_ = n + h", ""},
		{"compare named C", "var h Handle\n_ = h == 0\n_ = h == nil", ""},
		{"shift named C", "n := uint(1)\nvar h Handle\n_ = h << n\n_ = 1 << h", ""},
		{"inc named C", "var h Handle\nh++\nh += 2\nh <<= 1", ""},
		{"index named C", "n := 1\nvar h Handle\n_ = h[n]\n_ = h[n:]", ""},
		{"len named C", "var v V\n_ = len(v)", ""},
		{"call named C", "n := 1\nvar h Handle\n_ = h(n)", ""},
		{"indirect named C", "var h Handle\n_ = *h", ""},
		{"select named C", "var h Handle\n_ = h.x", ""},
		{"switch named C", "var h Handle\nswitch h {\ncase 1:\n}", ""},
		{"type switch named C",
			"var h H\nswitch x := h.(type) {\ncase fmt.Stringer:\n\t_ = x\ncase int, bool:\n" +
				"\t_ = x\n}",
			""},
		{"make named C sizes", "n := 2\n_ = make(V, n, n+1)", ""},
		{"assign const to named C", "var h Handle = 3\nh = 4\n_ = h", ""},
		{"arguments and results",
			"f := func(h Handle) Handle { return 0 }\n_ = f(3)\n" +
				"g := func(hs ...Handle) {}\ng(3, 4)",
			""},
		{"literals of named C",
			"var h Handle\n_ = []Handle{1, h}\n_ = [2]Handle{1}\n_ = struct{ h Handle }{h: 3}\n" +
				"_ = struct{ h Handle }{3}",
			""},
		{"map key named C", "n := 2\nm := map[Handle]int{1: n}\n_ = m[3]\nm[4] = 1", ""},
		{"send named C", "var c chan Handle\nc <- 1", ""},
		{"conversions named C",
			"n := 1\nvar h Handle\nvar p P\n_ = Handle(n)\n_ = Handle(3) + h\n_ = string(h)\n" +
				"_ = Handle(p)",
			""},
		{"named C struct", "var p P\np.x = 1\np.x++\n_ = &p\n_ = p.x * p.y\n_ = p.Len()", ""},
		{"pointer to named C", "var pp *P\n_ = pp.x\nqq := &P{}\nqq.x = 2", ""},
		{"named C field", "type G struct{ h Handle }\nvar g G\n_ = g.h + 1", ""},
		{"named C of other package", "var h lib.Handle\n_ = h + 1\n_ = -h", ""},
		{"pointer to opaque literal",
			"p := &std.vector[int]{}\np.push_back(1)\nq := &lib.CB\n_ = q.x",
			""},

		// The same shapes on Go values: the errors stay.
		{"go range", "for range 1.5 {\n}", "cannot range over 1.5 (untyped float constant)"},
		{"go elided literal",
			"_ = Undefined{{1, 2}}",
			"undefined: Undefined\nmissing type in composite literal"},
		{"go const conversion",
			"var u unsafe.Pointer\n_ = u == 1",
			"cannot convert 1 (untyped int constant) to type unsafe.Pointer"},
		{"go assign",
			"var g GoInt = \"s\"\n_ = g",
			`cannot use "s" (untyped string constant) as GoInt value in variable declaration`},
		{"go unary",
			"var g GoS\n_ = -g",
			"invalid operation: operator - not defined on g (variable of struct type GoS)"},
		{"go mismatched",
			"var g GoInt\nvar n int\n_ = g + n",
			"invalid operation: g + n (mismatched types GoInt and int)"},
		{"go inc", "var s string\ns++", "invalid operation: s++ (non-numeric type string)"},
		{"go indirect",
			"var n int\n_ = *n",
			"invalid operation: cannot indirect n (variable of type int)"},
		{"go index", "var n int\n_ = n[0]", "cannot index n (variable of type int)"},
		{"go slice", "var n int\n_ = n[1:2]", "cannot slice n (variable of type int)"},
		{"go shift count",
			"var f float64\n_ = 1 << f",
			"invalid operation: shift count f (variable of type float64) must be integer"},
		{"go shift operand",
			"var f float64\n_ = f << 1",
			"invalid operation: shifted operand f (variable of type float64) must be integer"},
		{"go selector", "var g GoS\n_ = g.y", "g.y undefined (type GoS has no field or method y)"},
		{"go make",
			"_ = make(GoInt, 1)",
			"invalid argument: cannot make GoInt: type must be slice, map, or channel"},
		{"go assert",
			"var n int\n_ = n.(fmt.Stringer)",
			"invalid operation: n (variable of type int) is not an interface"},
		{"go conversion",
			"var g GoS\n_ = GoInt(g)",
			"cannot convert g (variable of struct type GoS) to type GoInt"},
		{"go type switch",
			"var n int\nswitch n.(type) {\n}",
			"n (variable of type int) is not an interface"},
		{"go call",
			"var n int\nn()",
			"invalid operation: cannot call n (variable of type int): int is not a function"},
		{"go index value",
			"xs := []int{1}\ns := \"a\"\n_ = xs[s]\n_ = xs[s:]",
			"invalid argument: index s (variable of type string) must be integer\n" +
				"invalid argument: index s (variable of type string) must be integer"},
		{"go make size",
			"s := \"x\"\n_ = make([]int, s)",
			"invalid argument: index s (variable of type string) must be integer"},
		{"go append",
			"var ns []int\nns = append(ns, \"x\")",
			`cannot use "x" (untyped string constant) as int value in argument to append`},
		{"go delete",
			"m := map[int]bool{}\ndelete(m, \"x\")",
			`cannot use "x" (untyped string constant) as int value in argument to delete`},
		{"go copy",
			"var a []int\nvar b []string\n_ = copy(a, b)",
			"invalid copy: arguments a (variable of type []int) and b (variable of type []string) " +
				"have different element types int and string"},
		{"go min",
			"var b bool\n_ = min(b, true)",
			"invalid argument: b (variable of type bool) cannot be ordered"},
		// A Go type over an undefined name is invalid by an error, not
		// from C/C++.
		{"go invalid named",
			"type T Undefined\nn := 1\n_ = make(T, n)",
			"undefined: Undefined\ndeclared and not used: n\n" +
				"invalid argument: cannot make T: type must be slice, map, or channel"},

		// Indexes, sizes and builtin arguments of C values.
		{"index by named C", "xs := []int{1}\nvar a [3]int\ns := \"ab\"\nvar h Handle\n" +
			"_ = xs[h]\n_ = xs[h:]\n_ = a[h]\n_ = s[h]\nxs[h] = 1\n_ = [3]int{}[h]", ""},
		{"index by other package", "xs := []int{1}\n_ = xs[lib.CB]\n_ = xs[lib.CB:]", ""},
		{"make sizes of C",
			"var h Handle\n_ = make([]int, h)\n_ = make([]int, 1, h)\n_ = make([]int, lib.CB)",
			""},
		{"append named C", "var hs []Handle\nhs = append(hs, 1)\nhs = append(hs, 1, 2)", ""},
		{"delete named C", "m := map[Handle]int{}\ndelete(m, 1)", ""},
		{"copy named C", "var hs []Handle\nvar ns []int\n_ = copy(hs, ns)\n_ = copy(ns, hs)", ""},
		{"min max named C", "var h Handle\n_ = min(h, 1)\n_ = max(h, h)\n_ = min(lib.CB, 1)", ""},

		// Real errors next to C values stay.
		{"error inside",
			"_ = lib.CB[undefinedIndex]\nvar h Handle\n_ = h + undefinedRight",
			"undefined: undefinedIndex\nundefined: undefinedRight"},
		// A Go slice, map, array or pointer of a named type over a C/C++
		// type is a Go value: the errors of operations on it stay (pointer
		// arithmetic would compile in C++).
		{"go slice of named C inc", "var hs []Handle\nhs++",
			"invalid operation: hs++ (non-numeric type []Handle)"},
		{"go slice of named C unary", "var hs []Handle\n_ = -hs",
			"invalid operation: operator - not defined on hs (variable of type []Handle)"},
		{"go slice of named C less", "var hs []Handle\n_ = hs < hs",
			"invalid operation: hs < hs (operator < not defined on slice)"},
		{"go slice of named C indirect", "var hs []Handle\n_ = *hs",
			"invalid operation: cannot indirect hs (variable of type []Handle)"},
		{"go slice of named C call", "var hs []Handle\nhs()",
			"invalid operation: cannot call hs (variable of type []Handle): []Handle is not a function"},
		{"go slice of named C compare", "var hs []Handle\nvar ns []int\n_ = hs == ns",
			"invalid operation: hs == ns (mismatched types []Handle and []int)"},
		{"go map of named C compare", "var hm map[Handle]int\n_ = hm == hm",
			"invalid operation: hm == hm (map can only be compared to nil)"},
		{"go array of named C plus", "var a [2]Handle\nn := 1\n_ = a + n",
			"invalid operation: a + n (mismatched types [2]Handle and int)"},
		{"go pointer arithmetic plus", "var pp *P\n_ = pp + 1",
			"invalid operation: pp + 1 (mismatched types *P and untyped int)"},
		{"go pointer arithmetic minus", "var pp, qq *P\n_ = pp - qq",
			"invalid operation: operator - not defined on pp (variable of type *P)"},
		{"go pointer inc", "var pp *P\npp++", "invalid operation: pp++ (non-numeric type *P)"},

		// A Go error inside an expression with a C operand that starts at
		// the same place (falseErrors.drop): the error is about the inner
		// expression, not about the C operand.
		{"go inside C compare", "n := 1\ns := \"a\"\nvar h Handle\n_ = n + s == h",
			"invalid operation: n + s (mismatched types int and string)"},
		{"go inside C less", "var g GoInt\nvar n int\nvar h Handle\n_ = g + n < h",
			"invalid operation: g + n (mismatched types GoInt and int)"},
		{"go const inside C and", "var u unsafe.Pointer\nvar h Handle\n_ = 1 == u && h == 0",
			"cannot convert 1 (untyped int constant) to type unsafe.Pointer"},
		{"go shift inside C sum", "var f float64\nvar h Handle\n_ = f<<1 + h",
			"invalid operation: shifted operand f (variable of type float64) must be integer"},
		{"go inside other package", "s := \"a\"\nn := 1\nif s+n == lib.CB {\n}",
			"invalid operation: s + n (mismatched types string and int)"},
		{"go typed inside C", "n, m := 1, 2\nvar h Handle\n_ = n + m == h", ""},
		{"go next to C", "var h Handle\n_ = h + 1\nvar u unsafe.Pointer\n_ = u == 1",
			"cannot convert 1 (untyped int constant) to type unsafe.Pointer"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := falseErrorMessages(t, c.body); got != c.want {
				t.Errorf("body:\n%s\nerrors:\n%s\nwant:\n%s", c.body, got, c.want)
			}
		})
	}
}

// TestFalseErrorsLabels checks that the expressions that a dropped false
// error leaves without a type are opaque, and that a value used only in a
// subtree that go/types skipped after such an error is typed.
func TestFalseErrorsLabels(t *testing.T) {
	body := `n := 1
var h Handle
_ = h + 1 // plus
_ = lib.CB[1:n] // slice
_ = lib.CB(n) // call
m := map[Handle]int{1: n} // map
_ = m
var v V
for i, x := range v {
	_, _ = i, x // range
}`
	src := strings.Replace(falseErrorsMain, "BODY", body, 1)
	prog, p := checkMain(t, map[string]string{"main.gp": src, "lib/lib.gp": falseErrorsLib})
	checkLabels(t, prog, p, src, []labelCase{
		{"// plus", "h", "h + 1", "opaque"},
		{"// plus", "h", "h", "typed"},
		{"// slice", "lib", "lib.CB[1:n]", "opaque"},
		{"// slice", "n", "n", "typed"},
		{"// call", "lib", "lib.CB(n)", "opaque"},
		{"// call", "n", "n", "typed"},
		{"// map", "n}", "n", "typed"},
		{"// range", "i", "i", "opaque"},
		{"// range", "x", "x", "opaque"},
	})
}

// TestFalseErrorsConversions checks that a dropped false error doesn't
// stop the rewrite of a conversion of the value it is about: an error at
// the argument of a conversion (collector.blocking) means that the value
// may be invalid because of a mistake, but a dropped one is no mistake. So
// the conversion is typed int, and the error after it is reported.
func TestFalseErrorsConversions(t *testing.T) {
	for _, body := range []string{
		// The errors at the arguments: mismatched types (n + h), and an
		// operator that go/types doesn't know on Handle.
		"n := 1\nvar h Handle\nk := int(n + h)",
		"var h Handle\nk := int(h % h)",
		// The error is inside the argument.
		"k := int((std.vector[std.vector[int]]{{1, 2}}).size())",
	} {
		got := falseErrorMessages(t, body+"\n_ = k + \"a\"")
		want := `invalid operation: k + "a" (mismatched types int and untyped string)`
		if got != want {
			t.Errorf("body:\n%s\nerrors:\n%s\nwant:\n%s", body, got, want)
		}
	}
}

// TestFalseErrorsBrokenImport checks that the false errors are not
// dropped in a package whose import has errors: there an invalid value
// can come from an error (see also TestConversionBrokenImport).
func TestFalseErrorsBrokenImport(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nimport \"<x.h>\"\n\nvar Y = undefinedY\n\nvar Z = x.z\n",
		"main.gp":   "package main\n\nimport \"example.com/m/a\"\n\nfunc main() {\n\t_ = *a.Y\n}\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m/a\n" +
		"a/a.gp:5:9: undefined: undefinedY\n" +
		"# example.com/m\n" +
		"./main.gp:6:7: invalid operation: cannot indirect a.Y (variable with invalid type)\n"
	if got := errorText(t, prog, r, root, true); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestFalseErrorCodes pins the go/types error codes of the false errors,
// which come from Go's internal/types/errors, through the errors that
// go/types reports for Go code with the same codes.
func TestFalseErrorCodes(t *testing.T) {
	for _, c := range []struct {
		body string
		code int
	}{
		{"var u unsafe.Pointer\n_ = u == 1", invalidConstVal},
		{"var g GoInt = \"s\"\n_ = g", incompatibleAssign},
		{"var g GoS\n_ = -g", undefinedOp},
		{"var g GoInt\nvar n int\n_ = g + n", mismatchedTypes},
		{"var s string\ns++", nonNumericIncDec},
		{"var n int\n_ = *n", invalidIndirection},
		{"var n int\n_ = n[0]", nonIndexableOperand},
		{"var n int\n_ = n[1:2]", nonSliceableOperand},
		{"var f float64\n_ = 1 << f", invalidShiftCount},
		{"var f float64\n_ = f << 1", invalidShiftOperand},
		{"_ = Undefined{{1, 2}}", untypedLit},
		{"var g GoS\n_ = g.y", missingFieldOrMethod},
		{"_ = make(GoInt, 1)", invalidMake},
		{"var n int\n_ = n.(fmt.Stringer)", invalidAssert},
		{"var g GoS\n_ = GoInt(g)", invalidConversion},
		{"var n int\nswitch n.(type) {\n}", invalidTypeSwitch},
		{"for range 1.5 {\n}", invalidRangeExpr},
		{"var n int\nn()", invalidCall},
		{"xs := []int{1}\ns := \"a\"\n_ = xs[s]", invalidIndex},
		{"var a []int\nvar b []string\n_ = copy(a, b)", invalidCopy},
		{"var b bool\n_ = min(b, true)", invalidMinMaxOperand},
	} {
		root := writeTree(t, map[string]string{
			"gocpp.mod":  "module example.com/m\n",
			"main.gp":    strings.Replace(falseErrorsMain, "BODY", c.body, 1),
			"lib/lib.gp": falseErrorsLib,
		})
		prog, r := loadAndCheck(t, load.Config{}, root, ".")
		var codes []int
		for _, d := range mainPkg(t, prog, r).Errors.Diagnostics() {
			if d.Severity == diag.Error {
				codes = append(codes, d.Code)
			}
		}
		if len(codes) == 0 || codes[len(codes)-1] != c.code {
			t.Errorf("%q: codes %v, want the last one %d", c.body, codes, c.code)
		}
	}
}

// TestFalseErrorsGoPackage checks a package without header imports that
// imports a package with C: the false errors on the other package's
// values are dropped there too, and a Go error inside an expression with
// such a value stays.
func TestFalseErrorsGoPackage(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"lib/lib.gp": falseErrorsLib,
		"main.gp": `package main

import "example.com/m/lib"

func main() {
	s, n := "a", 1
	_ = *lib.CB
	for range lib.CB {
	}
	if s+n == lib.CB {
	}
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./main.gp:10:5: invalid operation: s + n (mismatched types string and int)\n"
	if got := errorText(t, prog, r, root, true); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestFalseErrorsGoLabels checks the labels next to the errors that stay
// on Go containers and pointers of named types over C/C++ types: -hs,
// pp + 1 and hm == hm are not opaque (cOperand counts such a value only as
// the X of a selector and as an argument of delete and copy), while pp.x,
// delete(m, 1) and copy(hs, ns), whose false errors are dropped, are.
func TestFalseErrorsGoLabels(t *testing.T) {
	body := `var hs []Handle
var ns []int
var pp *P
var hm map[Handle]int
_ = -hs // neg
_ = pp + 1 // ptr
_ = hm == hm // map
_ = pp.x // sel
delete(hm, 1) // delete
_ = copy(hs, ns) // copy`
	src := strings.Replace(falseErrorsMain, "BODY", body, 1)
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    src,
		"lib/lib.gp": falseErrorsLib,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	p := mainPkg(t, prog, r)
	want := "invalid operation: operator - not defined on hs (variable of type []Handle)\n" +
		"invalid operation: pp + 1 (mismatched types *P and untyped int)\n" +
		"invalid operation: hm == hm (map can only be compared to nil)"
	var msgs []string
	for _, d := range p.Errors.Diagnostics() {
		msgs = append(msgs, d.Msg)
	}
	if got := strings.Join(msgs, "\n"); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	for _, c := range []labelCase{
		{"// neg", "-", "-hs", "neither"},
		{"// ptr", "pp", "pp + 1", "neither"},
		{"// map", "hm", "hm == hm", "neither"},
		{"// sel", "pp", "pp.x", "opaque"},
		{"// delete", "delete", "delete(hm, 1)", "opaque"},
		{"// copy", "copy", "copy(hs, ns)", "opaque"},
	} {
		e := findExpr(t, prog, p, lineOf(t, src, c.marker), c.at, c.src)
		if got := labelOf(p, e); got != c.want {
			t.Errorf("%s: %s is %s, want %s", c.marker, c.src, got, c.want)
		}
	}
}
