// phase: 05

// Type assertions go/types rejects: a non-interface operand, an impossible assertion, a
// duplicate case (step 5.5).
package main

type Shape interface{ Area() float64 }
type Rect struct{}

func (*Rect) Area() float64 { return 0 }

type Circle struct{}

func main() {
	var n int
	_ = n.(int) // ERROR "invalid operation: n \(variable of type int\) is not an interface"
	var s Shape
	_ = s.(Circle) // ERROR "impossible type assertion: s.\(Circle\)"
	_ = s.(Rect)   // ERROR "impossible type assertion: s.\(Rect\)"
	_ = s.(*Rect)
	var x any
	_ = x.(Rect)
	switch x.(type) {
	case int:
	case int: // ERROR "duplicate case int in type switch"
	}
	switch s.(type) {
	case Circle: // ERROR "impossible type switch case: Circle"
	}
}
