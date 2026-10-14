// Structs (Phase 03 step 3.8): struct types (package-level, local ones moved to the namespace,
// anonymous ones with one C++ type per distinct type), zero values, literals (positional, keyed
// in source order with temporaries where C++'s order differs, nested, elided types, &T{...}),
// fields read and written (=, op=, ++), through pointers (also promoted through embedded fields
// and embedded pointers), &x.f with the owner table (boxed, package, through a pointer, a slice
// element, a call: gp::field_ptr), == and != (also with blank fields, floats and nested
// structs), value semantics (assignment, parameters, results copy), recursive types (a list
// and a tree through pointers and slices), the self-reachable assignment n = n.kids[0] (a copy
// first), fields as targets of parallel assignments and range loops (the operands are evaluated
// first), names that C++ reserves (fields named class, new, like their struct; a type named
// gp or std; a field named like its type), field tags, structs larger than 128 KiB, and
// printing structs with fmt (the shim's gp_fields).
package main

import "fmt"

type Point struct {
	X, Y int
}

// Rect is defined after Line in C++: Line holds a Rect by value.
type Line struct {
	R    Rect
	Name string
	_    int
	n    int64
}

type Rect struct {
	Min, Max Point
}

// Node is recursive through a pointer and a slice.
type Node struct {
	next *Node
	kids []Node
	val  int
}

// Inner is embedded by value, Mid by pointer, and Deep through Mid.
type Inner struct {
	A int
	b string
}

type Deep struct{ D int }

type Mid struct {
	*Deep
	M int
}

type Outer struct {
	Inner
	*Mid
	X int
}

// Names that C++ reserves.
type Words struct {
	Words int
	class string
	new   float64
	Point Point
	arr   [3]int
	nums  []int
}

type gp struct{ v int }
type std struct{ w uint8 }

type Big struct {
	data [1 << 15]int
	n    int
}

type Blank struct {
	a int
	_ string
	b float64
}

var gpt Point
var grect = Rect{Max: Point{3, 4}}
var gptr = &Point{7, 8}
var x = 1

func f(tag int) int {
	fmt.Print("f", tag, " ")
	x++
	gpt.X++
	return x
}

func mk() *Outer {
	return &Outer{X: 9, Mid: &Mid{Deep: &Deep{D: 4}}}
}

func pick(p *Point) *Point {
	fmt.Print("pick ")
	return p
}

func area(r Rect) int {
	r.Max.X -= r.Min.X // a copy: the caller's r doesn't change
	return r.Max.X * (r.Max.Y - r.Min.Y)
}

func grow(r Rect, d int) Rect {
	r.Max.X += d
	r.Max.Y += d
	return r
}

func sum(b Big) int {
	return b.n + b.data[5]
}

func push(n *Node, v int) *Node {
	return &Node{next: n, val: v}
}

// local and local2 declare different types L: the anonymous struct types struct{ l L } are
// different too.
func local() {
	type L struct{ a int }
	type Point struct{ c bool }
	fmt.Println(L{1}, Point{true}, struct{ l L }{L{2}})
}

func local2() {
	type L struct{ b string }
	fmt.Println(L{"z"}, struct{ l L }{L{"y"}})
}

func main() {
	// Zero values and literals.
	var p Point
	q := Point{1, 2}
	r := Point{Y: 5}
	fmt.Println(p, q, r, gpt, grect, *gptr)
	fmt.Println(Point{Y: f(1), X: f(2)}, Point{f(3), f(4)}, Point{Y: x, X: f(5)}, gpt)
	rects := []Rect{{Min: Point{1, 1}}, {Max: Point{X: 2}}}
	ptrs := []*Point{{1, 2}, {3, 4}}
	fmt.Println(rects[0], rects[1], *ptrs[1], len(ptrs))

	// Fields, through pointers, op= and ++.
	q.X = 10
	q.Y += 3
	q.X++
	pp := &q
	pp.Y *= 2
	pp.X--
	fmt.Println(q, pp.X, *pp)
	grect.Min.Y = 2
	gptr.X += 10
	pick(&q).Y++
	fmt.Println(grect, *gptr, q)

	// &x.f: a boxed local, a package variable, through a pointer, a slice element, a call.
	b := Point{1, 2}
	py := &b.Y
	*py = 20
	pg := &grect.Max.X
	*pg = 30
	px := &pp.X
	*px = 40
	ps := []Point{{5, 6}}
	pe := &ps[0].X
	*pe = 50
	pm := &mk().X
	pd := &mk().Mid.M
	fmt.Println(b, grect, q, ps[0], *pm, *pd)

	// Comparisons.
	zero := 0.0
	nan := struct{ f float64 }{zero / zero}
	fmt.Println(q == *pp, q != b, Rect{} == Rect{}, nan == nan, nan != nan)
	fmt.Println(struct{ z float64 }{0} == struct{ z float64 }{-zero}, Blank{a: 1, b: 2} == Blank{a: 1, b: 2})

	// Value semantics.
	r1 := Rect{Point{0, 0}, Point{4, 3}}
	r2 := r1
	r2.Max.X = 100
	fmt.Println(area(r1), r1, r2, grow(r1, 1), r1 == r2)
	var l Line
	l.R = r1
	l.Name = "l"
	l.n = 1 << 40
	l2 := l
	l2.R.Min.X = 9
	fmt.Println(l, l2 == l, l2.R.Min)

	// Embedded fields: promoted fields, through embedded pointers.
	var o Outer
	o.A = 1
	o.b = "x"
	o.Mid = &Mid{M: 2}
	o.M++
	po := &o
	po.A += 5
	po.Inner.A *= 2
	o.Deep = &Deep{D: 7}
	fmt.Println(o.A, o.b, o.M, o.Inner, o.X, po.D, mk().D, mk().X)
	pa := &o.Inner.A
	*pa = 100
	pdd := &o.D // through the embedded pointers: shares o.Deep's owner
	*pdd = 70
	fmt.Println(o.A, o.Mid.Deep.D, *pdd)

	// Recursive types: a list and a tree; n = n.kids[0] copies the child first.
	var list *Node
	for i := range 4 {
		list = push(list, i)
	}
	for n := list; n != nil; n = n.next {
		fmt.Print(n.val, " ")
	}
	fmt.Println()
	tree := Node{val: 1}
	tree.kids = append(tree.kids, Node{val: 2, kids: []Node{{val: 3}, {val: 4}}})
	tree = tree.kids[0]
	fmt.Println(tree.val, len(tree.kids), tree.kids[1].val)
	tree.kids[0] = tree.kids[0]
	tree = *push(nil, 5)
	fmt.Println(tree.val, tree.next == nil)

	// Fields as targets: the operands of p.f and s[i].f are evaluated first.
	p1, p2 := &Point{1, 2}, &Point{3, 4}
	p1, p1.X = p2, 100
	fmt.Println(*p1, *p2)
	i := 0
	i, ps[i].Y = 1, 60
	ps = append(ps, Point{})
	for _, ps[1].X = range []int{7, 8} {
	}
	fmt.Println(ps[0], ps[1], i)
	q.X, q.Y = q.Y, q.X
	fmt.Println(q)
	// A target that is a part of a local (w.i, a[0]) is a write to it: the operands of the
	// targets after it that read it are evaluated first, as Go does; so is an embedded pointer.
	var wl Words
	wl.Words, ps[wl.Words].X = 1, 70
	la := [2]int{}
	la[0], ps[la[0]].Y = 1, 80
	fmt.Println(ps[0], ps[1], wl.Words, la[0])
	e0 := &Deep{}
	m := Mid{Deep: e0}
	m.Deep, m.D = &Deep{D: 9}, 5
	fmt.Println(e0.D, m.D)

	// Reserved names.
	w := Words{Words: 1, class: "c", new: 2.5, arr: [3]int{1, 2, 3}}
	w.arr[1] = 20
	w.nums = append(w.nums, 5)
	w.Point.X = 3
	fmt.Println(w, w.Words, w.class, w.new, gp{1}, std{2})

	// Anonymous structs, local types, tags.
	a1 := struct {
		X int `json:"x"`
	}{1}
	a2 := struct {
		X int `json:"x"`
	}{2}
	a1 = a2
	fmt.Println(a1, a1 == a2, struct{}{} == struct{}{})
	local()
	local2()

	// Structs larger than 128 KiB: boxed, and passed by reference.
	var big Big
	big.n = 3
	big.data[5] = 4
	bp := &Big{}
	bp.n = 8
	big2 := Big{n: f(6)}               // made in its box: the zero value, then n
	bp2 := &Big{[1 << 15]int{5: 1}, x} // positional
	fmt.Println(sum(big), sum(*bp), sum(big2), sum(*bp2))

	// Arrays of structs.
	arr := [2]Point{{1, 2}, {3, 4}}
	arr[1].Y++
	pa0 := &arr[0].X
	*pa0 = 9
	fmt.Println(arr[0], arr[1])
}
