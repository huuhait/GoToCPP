// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: main\.Shape is main\.Rect, not \*main\.Rect

// The panic names the static interface type of the operand: main.Shape is main.Rect, not
// *main.Rect (step 5.5).
package main

import "fmt"

type Shape interface{ Area() float64 }
type Rect struct{ w, h float64 }

func (r Rect) Area() float64 { return r.w * r.h }

func main() {
	var s Shape = Rect{1, 2}
	fmt.Println(s.Area())
	p := s.(*Rect)
	fmt.Println(p.w)
}

// Output:
// 2
