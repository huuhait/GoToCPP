package main

import "fmt"

type Shape interface{ Area() float64 }

type Rect struct{ w, h float64 }

func (r Rect) Area() float64 { return r.w * r.h }

type Circle struct{ r float64 }

func (c *Circle) Area() float64 { return 3 * c.r * c.r }

func describe(s Shape) string {
	switch v := s.(type) {
	case Rect:
		return fmt.Sprint("rect ", v.w)
	case *Circle:
		return fmt.Sprint("circle ", v.r)
	}
	return "?"
}

func main() {
	var a Shape = Rect{2, 3} // copies the Rect into a box
	var b Shape = &Circle{1} // shares the pointer's owner
	r, ok := a.(Rect)
	fmt.Println(a.Area(), b.Area(), describe(a), r.w, ok)
}
