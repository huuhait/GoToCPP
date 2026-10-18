// phase: 05
// ExitCode: 2
// Stderr: ^panic: interface conversion: main.Shape is \*main.Square, not main.Boxed\n\ngoroutine 1 \[running\]:\n

// Type switches and assertions with embedded types: the embedded struct, the outer struct and
// pointers to them are four different dynamic types, and the case order decides which one
// matches an interface case.
package main

import "fmt"

type Shape interface{ Area() float64 }

type Rect struct{ W, H float64 }

func (r Rect) Area() float64 { return r.W * r.H }

type Square struct {
	Rect
	label string
}

type Boxed struct {
	*Square
	depth float64
}

func (b Boxed) Area() float64 { return b.Square.Area() * b.depth }

func kind(x any) string {
	switch v := x.(type) {
	case Boxed:
		return fmt.Sprint("Boxed ", v.Area())
	case *Boxed:
		return fmt.Sprint("*Boxed ", v.Area())
	case Square:
		return fmt.Sprint("Square ", v.label)
	case *Square:
		return fmt.Sprint("*Square ", v.label)
	case Shape:
		return fmt.Sprint("Shape ", v.Area())
	}
	return "other"
}

func main() {
	r := Rect{2, 3}
	s := Square{r, "s"}
	b := Boxed{&s, 2}
	fmt.Println(kind(r), "|", kind(s), "|", kind(&s), "|", kind(b), "|", kind(&b), "|", kind(1))
	var sh Shape = b
	_, isBoxed := sh.(Boxed)
	_, isSquare := sh.(Square)
	fmt.Println(isBoxed, isSquare)
	sh = &s
	_, isPtr := sh.(*Square)
	_, isRect := sh.(Rect)
	fmt.Println(isPtr, isRect, sh.Area())
	defer func() {}()
	_ = sh.(Boxed)
}

// Output:
// Shape 6 | Square s | *Square s | Boxed 12 | *Boxed 12 | other
// true false
// true false 6
