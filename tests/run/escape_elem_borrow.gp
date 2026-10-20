// phase: 08

// Element references (step 15.2, ADR-114): v := xs[i] and for _, v := range xs with a pure body make
// v a const reference to the element: no copy, no count. The loops here call interface methods
// whose implementations are all pure (class hierarchy analysis) and read only.
package main

import "fmt"

type Shape interface {
	Area() int
}

type Rect struct{ w, h int }

func (r *Rect) Area() int { return r.w * r.h }

type Sq struct{ s int }

func (q Sq) Area() int { return q.s * q.s }

type Node struct {
	val  int
	next *Node
}

func total(shapes []Shape) int {
	sum := 0
	for _, s := range shapes {
		sum += s.Area()
	}
	return sum
}

func pick(vals []Shape, i int) int {
	v := vals[i]
	return v.Area() + 1
}

func names(xs []string) int {
	n := 0
	for _, s := range xs {
		n += len(s)
	}
	return n
}

func nodes(ns []*Node) int {
	sum := 0
	for _, n := range ns {
		sum += n.val
	}
	return sum
}

func chain(ns []*Node) int {
	sum := 0
	for i := range ns {
		n := ns[i]
		for n != nil {
			sum += n.val
			n = n.next
		}
	}
	return sum
}

func main() {
	shapes := []Shape{&Rect{2, 3}, Sq{4}}
	fmt.Println(total(shapes), pick(shapes, 1), names([]string{"ab", "cde"}))
	ns := []*Node{{val: 1}, {val: 2}}
	ns[0].next = ns[1]
	fmt.Println(nodes(ns), chain(ns))
}

// Output:
// 22 17 5
// 3 5
