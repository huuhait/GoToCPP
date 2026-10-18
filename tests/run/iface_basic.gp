// phase: 05

// Interfaces: a value, a pointer and a slice of interface values; value and pointer receivers
// (step 5.4). The shapes example of the plan.
package main

import "fmt"

type Shape interface{ Area() float64 }

type Rect struct{ W, H float64 }

func (r Rect) Area() float64 { return r.W * r.H }

type Circle struct{ R float64 }

func (c *Circle) Area() float64 { return 3.14159 * c.R * c.R }

type Square struct{ S float64 }

func (s Square) Area() float64 { return s.S * s.S }

func total(shapes []Shape) float64 {
	t := 0.0
	for _, s := range shapes {
		t += s.Area()
	}
	return t
}

func main() {
	shapes := []Shape{Rect{2, 3}, &Circle{1}, Square{2}, &Rect{1, 1}}
	for i, s := range shapes {
		fmt.Println(i, s.Area())
	}
	fmt.Println(total(shapes))
	var s Shape
	fmt.Println(s == nil)
	s = Square{3}
	fmt.Println(s != nil, s.Area())
	s = &Square{4}
	fmt.Println(s.Area())
}

// Output:
// 0 6
// 1 3.14159
// 2 4
// 3 1
// 14.14159
// true
// true 9
// 16
