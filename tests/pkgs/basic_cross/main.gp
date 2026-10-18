// phase: 06

// Packages that use each other (Phase 06 step 6.2): constants, types with methods, package variables, interfaces implemented by types of other packages, the same type name in three packages, descriptors of types of other packages (assertions and type switches), init order.

package main

import (
	"fmt"

	"example.com/shop/geom"
	"example.com/shop/shape"
)

type Square struct{ S float64 }

func (s Square) Area() float64 { return s.S * s.S }
func (s Square) Name() string  { return "square" }

type Item struct{ N int }

func (i Item) Describe() string { return "main item" }

func main() {
	p := geom.New(1, 2).Add(geom.Point{3, 4})
	p.Scale(2)
	fmt.Println(p, geom.Max, geom.Name, geom.Boiling)
	fmt.Println(geom.Origin, geom.Next(), geom.Next(), geom.Counter)
	geom.Counter = 10
	fmt.Println(geom.Next())
	geom.Registry["b"] = 2
	fmt.Println(len(geom.Registry))

	ss := []shape.Shape{shape.Rect{2, 3}, &shape.Circle{1}, Square{4}}
	fmt.Println(shape.Total(ss))
	for _, s := range ss {
		shape.Show(s)
	}
	ds := []shape.Describer{geom.Item{1}, shape.Item{"x"}, Item{2}}
	for _, d := range ds {
		fmt.Println(d.Describe())
	}
	h := geom.NewHidden(5)
	fmt.Println(h.N())
	var x interface{} = p
	if q, ok := x.(geom.Point); ok {
		fmt.Println("point", q)
	}
	switch v := x.(type) {
	case shape.Rect:
		fmt.Println("rect", v)
	case geom.Point:
		fmt.Println("geom point", v.X)
	}
}

// Output:
// init geom 0
// init shape
// (8,12) 10 geom 100
// (0,0) 1 2 2
// 11
// 2
// 25.14159
// rect 6
// circle 3.14159
// square 16
// geom item 1
// shape item x
// main item
// 5
// point (8,12)
// geom point 8
