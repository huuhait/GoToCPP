// phase: 03

// Methods and C++ names (step 3.10): a method declared before its type, recursion through a nil
// receiver, methods of a named slice (one named slice, like a package function and like the base
// class gp::slice), methods named like C++ words (new, delete) and like their type, a package
// variable named like a field that a method reads, fmt's hooks in fields (a pointer-only Error, a
// String), several results as the arguments, a value receiver whose address is taken (a box), one
// larger than 128 KiB, a method of a map's entry and of a slice's element, and a deferred method
// call in a method with a named result.
package main

import "fmt"

// Uses a type declared later.
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

func (e *Elem) Sum() int {
	if e == nil {
		return 0
	}
	return e.v + e.next.Sum()
}

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

var X = 100

type Words struct {
	X     int
	class string
}

func (w Words) new() int     { return w.X + X }
func (w *Words) delete() int { return X - w.X }
func (w Words) Words() int   { return 7 }
func (w Words) String() string {
	return "W" + w.class
}

type Err struct{ code int }

func (e *Err) Error() string { return "err" + string(rune('0'+e.code)) }

type Box struct {
	E  *Err
	W  Words
	PW *Words
}

func two() (int, int) { return 3, 4 }

type Pt struct{ X, Y int }

func (p Pt) Plus(dx, dy int) Pt { return Pt{p.X + dx, p.Y + dy} }
func (p *Pt) Set(x, y int)       { p.X, p.Y = x, y }

func (p Pt) Addr() bool {
	q := &p
	q.X = 99
	return p.X == 99
}

type Big struct {
	a [20000]int
	n int
}

func (b Big) N() int { return b.n + len(b.a) }
func (b *Big) Inc()  { b.n++ }

func (p Pt) deferred() (r int) {
	defer p.Set(1, 2)
	defer func2(&r)
	return p.X
}

func func2(r *int) { *r += 10 }

func main() {
	var l List
	l.Push(1)
	l.Push(2)
	l.Push(3)
	fmt.Println(l.Len(), l.head.Sum())
	ids := IDs{1, 2}
	ids.Add(3)
	fmt.Println(ids.Sum(), ids.slice(), len(ids))
	w := Words{1, "c"}
	fmt.Println(w.new(), w.delete(), w.Words(), w)
	b := Box{E: &Err{3}, W: w, PW: &w}
	fmt.Println(b)
	var e Err
	e.code = 4
	fmt.Println(e.Error())
	p := Pt{1, 2}
	fmt.Println(p.Plus(two()), p.Plus(1, 1).Plus(2, 2))
	p.Set(two())
	fmt.Println(p, p.Addr(), p)
	var big Big
	big.Inc()
	fmt.Println(big.N())
	m := map[string]Pt{"a": {5, 6}}
	fmt.Println(m["a"].Plus(1, 1))
	fmt.Println(p.deferred())
	s := []Pt{{1, 1}}
	s[0].Set(7, 7)
	fmt.Println(s[0], s[0].Plus(1, 0))
}
