package load

import (
	"fmt"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
)

// reservedSrc has every kind of declaration named C. CheckReservedC must
// report exactly the ones marked with /*ERR*/ (decision 5 of job 1.6). The
// code doesn't have to type-check: the check only reads the syntax. Each
// marked C is on its own line, because diag drops an error with the same
// line and message as the error before it, like the go compiler.
const reservedSrc = `package p

import C "fmt" // ClassifyImports checks import names

func C/*ERR*/() {}

var C/*ERR*/ = 1
var a, C/*ERR*/ int

const C/*ERR*/ = 1
const (
	x = iota
	C/*ERR*/
)

type C/*ERR*/ int
type C/*ERR*/ = int

type L[C/*ERR*/ any] []int
type M[K comparable, C/*ERR*/ any] map[K]C

func tp[C/*ERR*/ any](
	C/*ERR*/ int,
) {
}

func params(a, C/*ERR*/ int, b ...C.int) (
	C/*ERR*/ string,
) {
	return
}

func variadic(C/*ERR*/ ...int) {}

func (C/*ERR*/ T) m1(
	C/*ERR*/ int,
) (
	C/*ERR*/ int,
) {
	return 0
}

func (l L[C/*ERR*/]) m2() {}

func (m *M[K, C/*ERR*/]) m3() {}

func (m *(M[K, C/*ERR*/])) m4() {}

func locals(xs []int, v any, ch chan int) {
	var C/*ERR*/ int
	const C/*ERR*/ = 2
	type C/*ERR*/ struct{}
	C/*ERR*/ := 1
	a, C/*ERR*/ := 1, 2
	for C/*ERR*/ := range xs {
	}
	for i, C/*ERR*/ := range xs {
	}
	for C/*ERR*/ := range 10 {
	}
	switch C/*ERR*/ := v.(type) {
	}
	select {
	case C/*ERR*/ := <-ch:
	case C/*ERR*/, ok := <-ch:
	}
	if C/*ERR*/ := f(); C > 0 {
	}
	for C/*ERR*/ := 0; C < 3; C++ {
	}
	switch C/*ERR*/ := 1; C {
	}
	_ = func(C/*ERR*/ int) (
		C/*ERR*/ string,
	) {
		return ""
	}
	go func(C/*ERR*/ int) {}(1)
	defer func() (C/*ERR*/ int) { return 0 }()
}

// Allowed: these names are not in a scope with C.

type S struct {
	C    int
	D, C int
}

type T int

func (T) C() {}

type I interface {
	C()
	M(C int) (C string)
}

type F func(C int) (C string)

var fv func(C int)

func uses() {
	var n C.int = C.int(1)
	_ = n
	_ = S{C: 1}
C:
	for {
		break C
	}
	goto C
	C = 2
	for C = range 3 {
	}
	y := struct{ C int }{C: 1}
	_ = y.C
}
`

// markedPositions returns the positions ("x.gp:line:col") of the C before
// each /*ERR*/ in src.
func markedPositions(src string) []string {
	var ps []string
	line, col := 1, 1
	for i := 0; i < len(src); i++ {
		if strings.HasPrefix(src[i:], "C/*ERR*/") {
			ps = append(ps, fmt.Sprintf("x.gp:%d:%d", line, col))
		}
		if src[i] == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return ps
}

func TestCheckReservedC(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", reservedSrc, 0)
	if err != nil {
		t.Fatal(err)
	}
	var l diag.List
	CheckReservedC(fset, f, &l)
	var got []string
	for _, d := range l.Diagnostics() {
		got = append(got, d.Pos.String())
		if d.Msg != "cannot declare C - C is reserved for C's built-in types" {
			t.Errorf("%s: message %q", d.Pos, d.Msg)
		}
		if d.End.Line != d.Pos.Line || d.End.Column != d.Pos.Column+1 {
			t.Errorf("%s: the error ends at %s, want the end of C", d.Pos, d.End)
		}
	}
	want := markedPositions(reservedSrc)
	if n := strings.Count(reservedSrc, "C/*ERR*/"); len(want) != n || n < 30 {
		t.Fatalf("markedPositions found %d markers, the source has %d", len(want), n)
	}
	for _, p := range want {
		if !slices.Contains(got, p) {
			t.Errorf("no error at %s", p)
		}
	}
	for _, p := range got {
		if !slices.Contains(want, p) {
			t.Errorf("unexpected error at %s", p)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d errors, want %d", len(got), len(want))
	}

	// The printed form of the first error.
	want0 := "x.gp:5:6: cannot declare C - C is reserved for C's built-in types\n"
	var first diag.List
	first.Add(l.Diagnostics()[0])
	if got := printDiags(t, &first); got != want0 {
		t.Errorf("printed %q, want %q", got, want0)
	}
}

// TestReservedPackageName checks the package clause: a package named C
// would bring a second C into the files that import it (finding 4 of
// round 1).
func TestReservedPackageName(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", "package C\n\nvar n C.int\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	var l diag.List
	CheckReservedC(fset, f, &l)
	want := "x.gp:1:9: invalid package name C - C is reserved for C's built-in types\n"
	if got := printDiags(t, &l); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	if d := l.Diagnostics()[0]; d.End.Column != 10 {
		t.Errorf("the error ends at %s, want the end of C", d.End)
	}
}

// TestCheckReservedCNone checks a file without a declaration named C that
// uses C.
func TestCheckReservedCNone(t *testing.T) {
	const src = `package p

func f(n C.int) C.size_t {
	x := C.int(1)
	return C.size_t(n + x)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var l diag.List
	CheckReservedC(fset, f, &l)
	if got := printDiags(t, &l); got != "" {
		t.Errorf("errors:\n%s", got)
	}
}
