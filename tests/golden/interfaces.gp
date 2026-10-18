// phase: 05

// Interfaces (steps 5.4 and 5.5): an interface struct and its wrappers, a named type's method
// tables, thunks and descriptor, the conversions (a value, a pointer, a constant, nil, an interface
// to another), a call, the assertions and a type switch.
package main

import "fmt"

type Shape interface{ Area() float64 }

type Namer interface {
	Shape
	Name() string
}

type Rect struct{ W, H float64 }

func (r Rect) Area() float64   { return r.W * r.H }
func (r Rect) Name() string    { return "rect" }
func (r *Rect) Scale(k float64) { r.W *= k }

func describe(x any) string {
	switch v := x.(type) {
	case nil:
		return "nil"
	case int, int64:
		return "int"
	case *Rect:
		return v.Name()
	case Shape:
		return "shape"
	}
	return "other"
}

func main() {
	var n Namer = &Rect{2, 3}
	var s Shape = n
	a := any(1)
	r, ok := s.(Rect)
	fmt.Println(s.Area(), describe(a), describe(nil), r.W, ok)
	fmt.Println(a.(int), n.Name())
}
