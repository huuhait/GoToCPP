// phase: 08

// Interface method calls whose receiver is a borrowed pointer (ADR-086): the thunk borrows the
// object from the interface value, which a method may replace while it runs (through a package
// variable or a field), a type switch and assertion, an interface holding a nil pointer, and an
// interface in a goroutine.
package main

import (
	"fmt"
	"sync"
)

type Shape interface {
	Area() int
	Grow(k int) int
	Swap() int
}

type Sq struct{ s int }
type Rect struct{ w, h int }

var cur Shape
var box struct{ s Shape }
var list []Shape

func (q *Sq) Area() int { return q.s * q.s }
func (q *Sq) Grow(k int) int {
	q.s += k
	return q.s
}

// Swap replaces every interface value that holds the receiver, then goes on using it.
func (q *Sq) Swap() int {
	cur = nil
	box.s = nil
	list[0] = nil
	q.s *= 2
	return q.Area()
}

func (r *Rect) Area() int { return r.w * r.h }
func (r *Rect) Grow(k int) int {
	r.h += k
	return r.h
}
func (r *Rect) Swap() int {
	cur = &Sq{1}
	r.w++
	return r.Area()
}

func total(shapes []Shape) (sum int) {
	for _, s := range shapes {
		sum += s.Area()
	}
	return
}

func main() {
	list = []Shape{&Sq{0}}
	cur = &Sq{3}
	fmt.Println(cur.Swap(), cur == nil)

	box.s = &Sq{4}
	fmt.Println(box.s.Swap(), box.s == nil)

	list = []Shape{&Sq{5}, &Rect{2, 3}}
	fmt.Println(list[0].Swap(), list[0] == nil)

	cur = &Rect{2, 2}
	fmt.Println(cur.Swap(), cur.Area())

	local := Shape(&Sq{6})
	fmt.Println(local.Grow(1), local.Area())

	var nilsq *Sq
	var n Shape = nilsq
	fmt.Println(n == nil, n != nil)
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil) }()
		n.Area()
	}()

	shapes := []Shape{&Sq{1}, &Rect{2, 3}, &Sq{2}}
	fmt.Println(total(shapes))
	for _, s := range shapes {
		switch v := s.(type) {
		case *Sq:
			v.Grow(1)
		case *Rect:
			v.Grow(2)
		}
	}
	fmt.Println(total(shapes))
	if q, ok := shapes[0].(*Sq); ok {
		fmt.Println(q.Grow(10), shapes[0].Area())
	}

	var wg sync.WaitGroup
	res := make([]int, 4)
	shared := Shape(&Sq{2})
	for i := range res {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res[i] = shared.Area() + i
		}()
	}
	wg.Wait()
	fmt.Println(res)
}

// Output:
// 36 true
// 64 true
// 100 true
// 6 1
// 7 49
// false true
// recovered: true
// 11
// 23
// 12 144
// [4 5 6 7]
