// Methods (Phase 03 step 3.10): value receivers (a copy at the entry: T v = *this; a box when
// the method takes its address or the type is larger than 128 KiB; a const T& when the body
// can't change the receiver: no calls, no stores but to its own variables), pointer receivers (static
// members with a gp::ptr), methods of named types that aren't structs, calls with the address
// taken (x.PM() on an addressable x: a boxed local, a package variable, a slice element, a field)
// and through pointers (p.M(), with the pointer held by a copy when it isn't a local), the
// receiver evaluated with the arguments in gc's order, methods declared before their types,
// recursion, defers of method calls (the receiver stored at the defer), named results, f(g())
// with several results, names that C++ reserves or that the members hide (a package variable
// and function named like a field or a method), fmt's String() and Error() through the hooks
// (value, pointer-only, in fields), and methods promoted through embedded fields and pointers.
package main

import "fmt"

// Len uses List, which is declared after it.
func (l List) Len() int {
	n := 0
	for e := l.head; e != nil; e = e.next {
		n++
	}
	return n
}

func (l *List) Push(v int) { l.head = &Elem{v, l.head} }

type List struct {
	head *Elem
}

type Elem struct {
	v    int
	next *Elem
}

// Sum recurses through the list, also on a nil receiver.
func (e *Elem) Sum() int {
	if e == nil {
		return 0
	}
	return e.v + e.next.Sum()
}

type Point struct{ X, Y int }

func (p Point) Add(q Point) Point {
	p.X += q.X // the copy changes, not the caller's Point
	p.Y += q.Y
	return p
}

func (p *Point) Move(dx, dy int) {
	p.X += dx
	p.Y += dy
}

// Moved takes the address of its receiver: the copy is boxed.
func (p Point) Moved() Point {
	q := &p
	q.Move(1, 1)
	return p
}

// Swap has named results and a deferred method call, whose receiver is stored at the defer.
func (p Point) Swap() (x, y int) {
	defer p.Move(100, 100) // p's box, at the defer
	defer show("deferred", p.Add(Point{}))
	p.X, p.Y = p.Y, p.X
	return p.X, p.Y
}

func show(s string, p Point) { fmt.Println(s, p) }

func two() (int, int) { return 3, 4 }

// IDs is a named type that is not a struct; slice is also the name of a package function and of
// the base class gp::slice in the struct of IDs.
type IDs []int

func (ids IDs) Sum() (s int) {
	for _, v := range ids {
		s += v
	}
	return
}

func (ids *IDs) Add(v int) { *ids = append(*ids, v) }

func slice(n int) int { return n * 2 }

func (ids IDs) slice() int { return slice(len(ids)) }

// Celsius has a String method, which fmt calls.
type Celsius float64

func (c Celsius) String() string { return itoa(int(c*10)/10) + "." + itoa(int(c*10)%10) + "°C" }

func (c Celsius) Fahrenheit() Celsius { return c*9/5 + 32 }

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + itoa(n%10)
}

// Names: X is a package variable and a field, class and new are C++ words, Words a method named
// like its type, and Point a field named like a type that a method's parameter uses.
var X = 100

type Words struct {
	X     int
	class string
	Point Point
}

func (w Words) new() int                 { return w.X + X }
func (w *Words) delete() int             { return X - w.X }
func (w Words) Words() int               { return 7 }
func (w Words) Shift(p Point) Point      { return p.Add(w.Point) }
func (w Words) String() string           { return "W" + w.class }
func (w Words) Error(code int) string    { return "not fmt's Error" }
func (w *Words) Reset(p Point) (q Point) { q, w.Point = w.Point, p; return }

// Err has Error only through a pointer: fmt calls it for a *Err (here a field of Box).
type Err struct{ code int }

func (e *Err) Error() string { return "err" + itoa(e.code) }

type Box struct {
	E  *Err
	W  Words
	PW *Words
	C  Celsius
}

// Big is larger than 128 KiB: a value receiver that changes is a box; one that the body can't
// change (N, which only reads it) is a const reference, as for the other read-only bodies.
type Big struct {
	a [20000]int
	n int
}

func (b Big) N() int      { return b.n + len(b.a) }
func (b *Big) Inc()       { b.n++ }
func (b Big) Bumped() int { b.n += 10; return b.n }

// Order: f changes the package variables that the receivers read.
type T struct{ v int }

func (t T) M(a int) int   { fmt.Println("M", t.v, a); return t.v }
func (t *T) PM(a int) int { fmt.Println("PM", t.v, a); return t.v }

var g = &T{1}
var gv = T{10}
var gs = []T{{100}, {200}}

func f() int { fmt.Println("f"); g = &T{2}; gv = T{20}; gs[0].v++; return 0 }
func h() *T  { fmt.Println("h"); return &T{3} }

// Promoted methods: Outer embeds Inner by value and Mid by pointer, Mid embeds *Deep.
type Inner struct{ v int }

func (i Inner) Get() int       { return i.v }
func (i *Inner) Set(v int)     { i.v = v }
func (i Inner) String() string { return "in" + itoa(i.v) }
func (d *Deep) Bump() int      { d.d++; return d.d }
func (d Deep) Val() int        { return d.d }
func (d *Deep) Error() string  { return "deep" + itoa(d.d) }

type Deep struct{ d int }

type Mid struct {
	*Deep
	m int
}

type Outer struct {
	Inner
	*Mid
	x int
}

func main() {
	var l List
	l.Push(1)
	l.Push(2)
	l.Push(3)
	fmt.Println(l.Len(), l.head.Sum())

	p := Point{1, 2}
	q := p.Add(Point{10, 20})
	fmt.Println(p, q)
	p.Move(5, 5)
	pp := &p
	pp.Move(1, 1)
	fmt.Println(*pp, pp.Add(Point{}), p.Moved(), p)
	fmt.Println(p.Add(Point{1, 1}).Add(Point{2, 2}))
	fmt.Println(p.Swap())
	fmt.Println(p)
	p.Move(two())
	fmt.Println(p)
	pts := []Point{{1, 1}, {2, 2}}
	pts[0].Move(7, 7)
	fmt.Println(pts[0], pts[1].Add(pts[0]))
	m := map[string]Point{"a": {5, 6}}
	fmt.Println(m["a"].Add(Point{1, 1}))

	ids := IDs{1, 2}
	ids.Add(3)
	fmt.Println(ids.Sum(), ids.slice(), len(ids))

	c := Celsius(21.5)
	fmt.Println(c, c.Fahrenheit(), c.String())
	for _, t := range []Celsius{-40, 0, 37} {
		fmt.Println(t, t.Fahrenheit())
	}

	w := Words{1, "c", Point{1, 1}}
	fmt.Println(w.new(), w.delete(), w.Words(), w.Shift(Point{2, 2}), w.Error(1), w)
	fmt.Println(w.Reset(Point{9, 9}), w.Point)
	b := Box{E: &Err{3}, W: w, PW: &w, C: 1.5}
	fmt.Println(b)
	fmt.Printf("%v %s\n", b, w)

	var big Big
	big.Inc()
	fmt.Println(big.N(), big.Bumped(), big.n)

	g.M(f())
	gv.M(f())
	g.PM(f())
	gv.PM(f())
	h().M(f())
	h().PM(f())
	gs[0].M(f())
	gs[1].PM(f())
	x := T{5}
	x.M(x.PM(1))
	px := &x
	px.M(px.PM(2) + x.PM(3))

	o := Outer{Inner{1}, &Mid{&Deep{2}, 3}, 4}
	fmt.Println(o.Get(), o.Val(), o.Bump(), o.Bump())
	o.Set(5)
	op := &o
	op.Set(6)
	fmt.Println(op.Get(), op.Val(), op.Bump(), o.Inner, o)
	defer fmt.Println("last", o.Get())
	defer o.Set(8)
	defer o.Bump()
}
