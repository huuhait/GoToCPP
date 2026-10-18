// phase: 08

// A range over a channel of interface values with a type switch in the body.
package main

import "fmt"

type shape interface{ area() float64 }

type sq struct{ s float64 }
type circ struct{ r float64 }

func (s sq) area() float64   { return s.s * s.s }
func (c circ) area() float64 { return 3 * c.r * c.r }

func main() {
	c := make(chan any, 8)
	c <- 1
	c <- "two"
	c <- 3.5
	c <- sq{2}
	c <- circ{1}
	c <- []int{1, 2}
	c <- nil
	c <- &sq{3}
	close(c)
	for v := range c {
		switch x := v.(type) {
		case int:
			fmt.Println("int", x+1)
		case string:
			fmt.Println("string", x+"!")
		case float64:
			fmt.Println("float", x*2)
		case shape:
			fmt.Printf("shape %T %.1f\n", x, x.area())
		case nil:
			fmt.Println("nil")
		default:
			fmt.Printf("other %T\n", x)
		}
	}
}

// Output:
// int 2
// string two!
// float 7
// shape main.sq 4.0
// shape main.circ 3.0
// other []int
// nil
// shape *main.sq 9.0
