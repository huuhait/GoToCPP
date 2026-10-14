// phase: 03

// Conversions between struct types (step 3.9): a named struct and an anonymous struct with the
// same fields convert implicitly both ways (Go allows it because one of them is unnamed), and a
// named struct over another (type B A) converts explicitly, field by field; (*B)(p) shares the
// struct.
package main

import "fmt"

type Point struct{ X, Y int }
type Pixel Point

func origin() struct{ X, Y int } {
	return struct{ X, Y int }{}
}

func shift(p Point, dx int) Point {
	p.X += dx
	return p
}

func main() {
	var p Point = origin()
	p = shift(struct{ X, Y int }{1, 2}, 10)
	fmt.Println(p, p == struct{ X, Y int }{11, 2})
	px := Pixel(p)
	px.Y = 99
	fmt.Println(px, p, Point(px))
	pp := (*Pixel)(&p)
	pp.X = -1
	fmt.Println(p)
	var anon struct{ X, Y int } = p
	anon.Y++
	fmt.Println(anon, p)
	pts := []Point{struct{ X, Y int }{3, 4}, {5, 6}}
	byPoint := map[Point]string{struct{ X, Y int }{3, 4}: "a"}
	fmt.Println(pts[0], pts[1], byPoint[pts[0]])
}

// Output:
// {11 2} true
// {11 99} {11 2} {11 99}
// {-1 2}
// {-1 3} {-1 2}
// {3 4} {5 6} a
