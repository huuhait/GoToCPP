// phase: 08

// Borrowed pointers (gp::bptr, ADR-086): a pointer parameter and a pointer receiver that the
// function only uses are raw pointers; the printers show them as the owning pointers.
package main

import "fmt"

type Point struct{ X, Y int }

func (p *Point) Sum() int {
	fmt.Println(p.X, p.Y) // @recv
	return p.X + p.Y
}

func peek(p *Point, q *Point) int {
	return p.Sum() + q.X // @param
}

func main() {
	a := &Point{3, 4}
	var none *Point
	fmt.Println(peek(a, a), none == nil)
}
