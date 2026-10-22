package check

import (
	"strings"
	"testing"
)

// TestNamedContainersOfCElements checks a named gocpp array, slice, map, pointer or func type with
// C/C++ parts (type Chars [1]C.char): what is taken out of a value of it has no type in go/types,
// and used to be "internal error: expression has no type: a[0]". It is opaque now, like the
// same expression for the unnamed type, and the program has no error.
func TestNamedContainersOfCElements(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, decl, body string }{
		{"array index", "type Chars [1]C.char", "var a Chars\n\ta[0] = 100"},
		{"array index read", "type Chars [1]C.char", "var a Chars\n\tprintln(a[0])"},
		{"array of arrays", "type Many [2][2]C.int", "var m Many\n\tm[1][1] = 3"},
		{"array slice", "type Chars [4]C.char", "var a Chars\n\tb := a[1:3]\n\t_ = b"},
		{"array range value", "type Chars [2]C.char", "var a Chars\n\tfor i, v := range a {\n\t\tprintln(i, v)\n\t}"},
		{"array range index", "type Chars [2]C.char", "var a Chars\n\tfor i := range a {\n\t\tprintln(i)\n\t}"},
		{"slice index", "type List []C.int", "var l List\n\tl = append(l, 3)\n\tprintln(l[0])"},
		{"slice range value", "type List []C.int", "var l List\n\tfor _, v := range l {\n\t\tprintln(v)\n\t}"},
		{"map index", "type Dict map[string]C.int", "var d Dict\n\td[\"x\"] = 1\n\tprintln(d[\"x\"])"},
		{"pointer deref", "type Ptr *C.int", "var p Ptr\n\tprintln(*p)"},
		{"func call", "type Fn func() C.int", "var f Fn\n\tprintln(f())"},
		{"instance of a generic slice", "type Box[T any] []T", "var b Box[C.int]\n\tb = append(b, 1)\n\tprintln(b[0])"},
		{"instance of a generic array", "type Pair[T any] [2]T", "var p Pair[C.int]\n\tp[1] = 3\n\tprintln(p[0])"},
		{"type parameter with a core type", "func get[T ~[]C.int](x T) C.int { return x[0] }", "var l []C.int\n\tprintln(get(l))"},
		{"type parameter with an array core type", "func set[T ~[2]C.int](x T) T {\n\tx[0] = 1\n\treturn x\n}", "var a [2]C.int\n\t_ = set(a)"},
		{"type parameter with a range", "func sum[T ~[]C.int](x T) C.int {\n\tvar s C.int\n\tfor _, v := range x {\n\t\ts += v\n\t}\n\treturn s\n}", "println(sum([]C.int{1}))"},
		{"copy of the array", "type Chars [1]C.char", "var a Chars\n\tb := a\n\tb[0] = 1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := "package main\n\n" + c.decl + "\n\nfunc main() {\n\t" + c.body + "\n}\n"
			if got := checkErrors(t, LastPhase, src); got != "" {
				t.Errorf("errors:\n%s", got)
			}
		})
	}
}

// TestNamedContainersOfGoElements checks that the errors of a named container with Go elements
// stay: nothing of the rule above is for a type without C/C++ parts.
func TestNamedContainersOfGoElements(t *testing.T) {
	t.Parallel()
	const src = `package main

type Bytes [1]byte
type List []int

func main() {
	var a Bytes
	a[0] = "x"
	var l List
	println(l["i"])
}
`
	got := checkErrors(t, LastPhase, src)
	for _, want := range []string{
		`./main.gp:8:9: cannot use "x" (untyped string constant) as byte value in assignment`,
		`./main.gp:10:12: cannot convert "i" (untyped string constant) to type int`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("errors:\n%s\nwant a line with %s", got, want)
		}
	}
	if strings.Contains(got, "internal error") {
		t.Errorf("errors:\n%s\nwant no internal error", got)
	}
}
