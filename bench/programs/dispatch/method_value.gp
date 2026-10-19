// bench: method_value
// category: dispatch
// size: 50000000
// smoke: 50000
// twin: no
// about: method values (bound receivers) and method expressions called in loops
package main

import "fmt"

type acc struct{ total int }

func (a *acc) add(x int)       { a.total += x }
func (a acc) scaled(k int) int { return a.total * k }

type vec struct{ x, y int }

func (v vec) dot(w vec) int { return v.x*w.x + v.y*w.y }

func main() {
	n := benchN
	a := &acc{}
	add := a.add // method value, receiver bound
	for i := 0; i < n; i++ {
		add(i & 15)
	}
	scaled := a.scaled // copies the receiver
	var sum uint64
	for i := 0; i < n; i++ {
		sum += uint64(scaled(i & 7))
	}

	addExpr := (*acc).add // method expression
	for i := 0; i < n; i++ {
		addExpr(a, i&3)
	}
	dot := vec.dot
	v := vec{3, 4}
	for i := 0; i < n; i++ {
		sum += uint64(dot(v, vec{i & 7, 1}))
	}
	bound := v.dot
	for i := 0; i < n; i++ {
		sum += uint64(bound(vec{1, i & 3}))
	}
	fmt.Println(a.total, sum)
}
