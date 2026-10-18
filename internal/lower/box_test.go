package lower

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// boxSrc is the program of TestBoxedVars. A variable whose declaration line ends with
// "// box: <reason>" must be boxed for that reason (boxReason.String), every other variable
// must not. It uses features of later steps (structs, methods, arrays, slices, closures): the
// analysis works on the typed AST, so it is checked without the phase gates.
const boxSrc = `package main

type T struct {
	n   int
	arr [3]int
	in  Inner
	ip  *Inner
}

type Inner struct{ v int }

func (p *Inner) Inc()   { p.v++ }
func (i Inner) Get() int { return i.v }
func (p *T) Bump()      { p.n++ }

func (t T) addr() *T { // t box: address
	return &t
}

var global int

func addr() {
	a := 1 // box: address
	b := 2
	_ = &a
	_ = b
	var t T // box: address
	_ = &t.n
	var u T // box: address
	_ = &u.arr[1]
	var w T // box: address
	_ = &(w.in).v
	var p *T
	_ = &p.n // through a pointer: p itself is not boxed
	s := []int{1}
	_ = &s[0] // the slice's array, not s
	pa := &[2]int{}
	_ = &pa[1] // through a pointer to an array
	_ = &global
	q := new(int)
	_ = &*q // *q is what q points to
}

func params(x int, y int, z string) (r int, e int) { // x box: address; r box: address
	_ = &x
	_ = &r
	_ = y
	_ = z
	return
}

func methods() {
	var t T // box: pointer method
	t.Bump()
	var u T // box: pointer method
	u.in.Inc()
	var v T // box: pointer method
	f := v.in.Inc // a method value
	_ = f
	var w T
	w.in.Get() // a value method
	var x T
	x.ip.Inc() // through the pointer field: x is not boxed
	p := &Inner{}
	p.Inc() // p is a pointer
	var in Inner // box: pointer method
	in.Inc()
}

func sliced() {
	var a [4]int // box: sliced
	_ = a[1:]
	var t T // box: sliced
	_ = t.arr[:]
	pa := &[2]int{}
	_ = pa[:] // a pointer to an array
	s := []int{1, 2}
	_ = s[1:]
	str := "abc"
	_ = str[1:]
}

func closures() {
	a := 1 // box: captured and changed
	b := 2
	c := 3 // box: captured and changed
	d := 4 // box: captured and changed
	var t T // box: captured and changed
	var arr [2]int // box: captured and changed
	s := []int{1}
	var p *T
	e := 5 // box: captured and changed
	n := 0 // box: captured and changed
	var k int // box: captured and changed
	f := func() int {
		a++
		t.n = 1
		s[0] = 2 // a write through the slice: s is not changed
		p.n = 3  // through the pointer
		return b + d + arr[0] + e
	}
	c += 1
	_ = func() { _ = c }
	d = 7
	arr[1] = 9
	for e = range 3 {
	}
	g := func() {
		h := func() { n = 1 }
		h()
	}
	k, m := 1, 2 // k exists: := changes it
	_ = func() { _ = k }
	_, _, _, _, _ = f, g, m, s, n
	o := 1
	_ = func() { _ = o } // captured, never changed
	var big [17]int // box: captured and large
	var small [16]int // 128 bytes: copied
	_ = func() { _, _ = big[0], small[0] }
}

func inner() {
	_ = func() {
		x := 1 // box: captured and changed
		y := 2 // a variable of the literal itself
		y++
		_ = func() { x++ }
	}
}

func loops() {
	for i := 0; i < 3; i++ { // i box: address
		_ = &i
	}
	for j := range 3 { // j box: address
		_ = &j
	}
}

func large() {
	var a [16384]int // exactly 128 KB: on the stack, as in Go
	var b [16385]int // box: large
	var c [2][1 << 20]byte // box: large
	type big struct {
		x [1 << 17]byte
		y int
	}
	var d big // box: large
	e := b // box: large
	_, _, _, _ = a, c, d, e
}

func largeParams(p [1 << 20]byte) (r [1 << 20]byte) { // parameters and results stay (Go)
	return p
}

func deferred() (r int) { // r box: captured and changed
	defer func() { _ = r }()
	return 5
}

func results() (a, b int) { // a box: captured and changed; b box: captured and changed
	_ = func() int { return a + b }
	return 1, 2
}

func litResults() {
	_ = func() (r int) { // r box: captured and changed
		_ = func() int { return r }
		return 3
	}
	_ = func() (r int) { // the return assigns r, but no closure captures it
		return 4
	}
}

func main() {}
`

// TestBoxedVars checks the boxing analysis (box.go): which variables of each function are
// boxed, and why.
func TestBoxedVars(t *testing.T) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(boxSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	file := pkg.Load.Files[0].AST

	// The expected boxes, by line: "// box: reason", or "// a box: reason; b box: reason"
	// for the variables of a func line.
	want := make(map[string]string) // "line:name" → reason
	for i, l := range strings.Split(boxSrc, "\n") {
		_, c, ok := strings.Cut(l, "// ")
		if !ok || !strings.Contains(c, "box: ") {
			continue
		}
		for _, part := range strings.Split(c, "; ") {
			name, reason, _ := strings.Cut(part, "box: ")
			want[fmt.Sprintf("%d:%s", i+1, strings.TrimSpace(name))] = reason
		}
	}

	got := make(map[string]string)
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		for v, r := range boxedVars(newTinfo(pkg.Info), fn) {
			line := fset.Position(v.Pos()).Line
			key := fmt.Sprintf("%d:%s", line, v.Name())
			// A declaration line with one variable names none in the comment.
			if _, ok := want[key]; !ok {
				if _, ok := want[fmt.Sprintf("%d:", line)]; ok {
					key = fmt.Sprintf("%d:", line)
				}
			}
			got[key] = r.String()
		}
	}
	var keys []string
	for k := range want {
		keys = append(keys, k)
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		if got[k] != want[k] {
			t.Errorf("line %s: boxed for %q, want %q", k, got[k], want[k])
		}
	}
}
