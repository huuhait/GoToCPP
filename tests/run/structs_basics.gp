// phase: 03

// Struct values: zero values, positional and keyed literals (the missing fields are zero), nested
// structs, fields read and written, == and != field by field, and copies on assignment, as
// parameters and as results.
package main

import "fmt"

type Point struct {
	X, Y int
}

type Rect struct {
	Min, Max Point
	Name     string
}


func width(r Rect) int {
	r.Max.X -= r.Min.X // r is a copy
	return r.Max.X
}

func moved(r Rect, d int) Rect {
	r.Min.X += d
	r.Max.X += d
	return r
}

func main() {
	var z Rect
	fmt.Println(z, z.Name == "", z.Min == Point{})
	p := Point{1, 2}
	q := Point{Y: 5}
	r := Rect{Name: "r", Max: Point{X: 4, Y: 3}}
	fmt.Println(p, q, r)
	p.X = 10
	p.Y += 5
	p.X++
	r.Min.Y--
	fmt.Println(p, r.Min, r.Max.X*r.Max.Y)
	fmt.Println(p == Point{11, 7}, p != q, r == Rect{Name: "r"}, Rect{} == z)
	r2 := r
	r2.Max.X = 100
	fmt.Println(width(r), r.Max.X, r2.Max.X, moved(r, 2), r)
}

// Output:
// {{0 0} {0 0} } true true
// {1 2} {0 5} {{0 0} {4 3} r}
// {11 7} {0 -1} 12
// true true false true
// 4 4 100 {{2 -1} {6 3} r} {{0 -1} {4 3} r}
