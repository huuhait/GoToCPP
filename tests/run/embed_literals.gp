// phase: 05

// Composite literals with embedded fields: keyed by the type name, unkeyed, nested, with an
// embedded pointer, in slices, arrays and maps, with the type elided, and as &T{...}.
package main

import "fmt"

type A struct{ X, Y int }
type B struct {
	A
	Z int
}
type P struct {
	*A
	Name string
}
type Deep struct {
	B
	*P
}

func (a A) Sum() int { return a.X + a.Y }

func main() {
	b1 := B{A{1, 2}, 3}
	b2 := B{A: A{Y: 7}, Z: 1}
	b3 := B{Z: 9}
	b4 := B{A: A{1, 2}}
	fmt.Println(b1, b2, b3, b4, b1.Sum(), b2.Sum())
	p := P{&A{3, 4}, "p"}
	p2 := &P{A: &A{X: 5}, Name: "q"}
	p3 := P{Name: "nil A"}
	fmt.Println(*p.A, p.Sum(), p2.A.X, p2.Y, p3.A == nil, p3.Name)
	d := Deep{B{A{1, 1}, 2}, &P{&A{8, 9}, "deep"}}
	fmt.Println(d.B.X, d.P.X, d.Z, d.Name, d.B.Sum(), d.P.Sum())
	bs := []B{{A{1, 2}, 3}, {A: A{X: 4}}, {}}
	arr := [...]B{{Z: 1}, {A: A{5, 5}}}
	m := map[string]B{"a": {A{1, 1}, 1}, "b": {Z: 2}}
	pp := []*P{{&A{1, 2}, "x"}, {Name: "y"}}
	fmt.Println(bs, arr, len(m), m["a"], m["b"], pp[0].X, pp[1].A == nil, len(arr))
	x, y := A{1, 2}, A{Y: 5}
	bb := B{x, 3}
	x.X = 100
	bb.Y = y.Y
	fmt.Println(bb, x, y)
}

// Output:
// {{1 2} 3} {{0 7} 1} {{0 0} 9} {{1 2} 0} 3 7
// {3 4} 7 5 0 true nil A
// 1 8 2 deep 2 17
// [{{1 2} 3} {{4 0} 0} {{0 0} 0}] [{{0 0} 1} {{5 5} 0}] 2 {{1 1} 1} {{0 0} 2} 1 true 2
// {{1 5} 3} {100 2} {0 5}
