// phase: 07

// A value of a generic type used through interfaces: the pointer receiver set, the value set, and method tables.
package main

import "fmt"

type Shape interface{ Area() float64 }
type Resetter interface{ Reset() }

type Rect[T ~int | ~float64] struct{ W, H T }

func (r Rect[T]) Area() float64 { return float64(r.W * r.H) }
func (r *Rect[T]) Reset()       { r.W, r.H = 0, 0 }

type Sq[T ~int | ~float64] struct{ Rect[T] }

func main() {
	var shapes []Shape
	shapes = append(shapes, Rect[int]{2, 3}, &Rect[float64]{0.5, 4}, Sq[int]{Rect[int]{3, 3}}, &Sq[float64]{Rect[float64]{1.5, 1.5}})
	total := 0.0
	for _, s := range shapes {
		total += s.Area()
		if r, ok := s.(Resetter); ok {
			r.Reset()
			fmt.Print("reset ")
		}
	}
	fmt.Println(total)
	for _, s := range shapes {
		fmt.Printf("%T %v|", s, s.Area())
	}
	fmt.Println()
	var r Resetter = &Rect[int]{1, 1}
	_, isShape := r.(Shape)
	var v any = Rect[int]{}
	_, valReset := v.(Resetter)
	fmt.Println(isShape, valReset)
}

// Output:
// reset reset 19.25
// main.Rect[int] 6|*main.Rect[float64] 0|main.Sq[int] 9|*main.Sq[float64] 0|
// true false
