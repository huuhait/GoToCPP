// phase: 04

// Methods of gocpp types with C/C++ parameters and results: calls on a value and through a
// pointer, a method value and a method expression.
package main

import (
	"fmt"

	"<cmath>"
)

type Q struct{ n C.int }

func (q *Q) Put(n C.int) { q.n = n }

func (q Q) Size() C.int { return q.n }

func (q Q) Scale(d C.double) C.double { return cmath.floor(d) }

func main() {
	var q Q
	q.Put(5)
	pq := &q
	pq.Put(6)
	size := q.Size
	scale := Q.Scale
	fmt.Println(int(q.Size()), int(size()), float64(scale(q, 2.5)))
}

// Output:
// 6 6 2
