// phase: 05

// == of interface values across interface types and through nested interfaces (step 5.6): an
// interface that holds an interface value holds its dynamic value (there are no nested
// interface values), comparing different interface types compares the dynamic values, and
// the nil rules of typed nil pointers.
package main

import "fmt"

type Shape interface{ Area() int }
type Named interface{ Name() string }
type Both interface {
	Shape
	Named
}

type Sq struct{ s int }

func (q Sq) Area() int    { return q.s * q.s }
func (q Sq) Name() string { return "sq" }

type Pt struct{ x int }

func (p *Pt) Area() int    { return p.x }
func (p *Pt) Name() string { return "pt" }

type Pair struct {
	a Shape
	b any
	c error
}

type MyErr struct{ code int }

func (e MyErr) Error() string { return fmt.Sprint("err", e.code) }

func main() {
	var both Both = Sq{3}
	var shape Shape = both
	var named Named = both
	var a any = shape
	var b any = named
	fmt.Println(a == b, shape == both, both == shape, a == both, named == shape.(Named))
	fmt.Println(shape == Sq{3}, shape == Sq{4}, a == Sq{3})

	// The conversion of a nil interface value is a nil interface value.
	var ns Shape
	var na any = ns
	fmt.Println(na == nil, ns == nil, na == ns, ns == nil)

	// A typed nil pointer is not nil, in any interface type.
	var p *Pt
	var sp Shape = p
	var ap any = sp
	fmt.Println(sp == nil, ap == nil, sp == ap, ap == (*Pt)(nil), sp == Shape((*Pt)(nil)))
	var bp Both = p
	fmt.Println(bp == nil, bp == sp, Named(bp) == Named(p))

	// Pointers: equal when they point to the same variable.
	p1, p2 := &Pt{1}, &Pt{1}
	var s1, s2, s3 Shape = p1, p2, p1
	fmt.Println(s1 == s2, s1 == s3, *p1 == *p2)

	// Structs with interface fields of three kinds.
	x := Pair{Sq{1}, 2, MyErr{3}}
	y := Pair{Sq{1}, 2, MyErr{3}}
	z := Pair{Sq{1}, 2.0, MyErr{3}}
	fmt.Println(x == y, x == z, x != z)
	var ax, ay, az any = x, y, z
	fmt.Println(ax == ay, ax == az)

	// error values
	var e1 error = MyErr{1}
	var e2 error = MyErr{1}
	var e3 error = MyErr{2}
	var ea any = e1
	fmt.Println(e1 == e2, e1 == e3, ea == e2, ea == MyErr{1}, e1 == nil)
	var enil error
	fmt.Println(enil == nil, any(enil) == nil, any(enil) == e1)

	// Arrays and comparisons across the same dynamic type but different interface types.
	arr1 := [2]Shape{Sq{1}, p1}
	arr2 := [2]Shape{Sq{1}, p1}
	arr3 := [2]Shape{Sq{1}, p2}
	fmt.Println(arr1 == arr2, arr1 == arr3)
	var aa1, aa2 any = arr1, arr2
	fmt.Println(aa1 == aa2, aa1 == any(arr3))
}

// Output:
// true true true true true
// true false true
// true true true true
// false false true true true
// false true true
// false true true
// true false true
// true false
// true false true true false
// true true false
// true false
// true false
