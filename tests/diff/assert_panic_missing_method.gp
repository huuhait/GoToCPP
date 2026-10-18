// phase: 05

// An assertion to an interface type whose method the dynamic type lacks panics with "missing
// method" (step 5.5).
package main

import "fmt"

type Shape interface{ Area() float64 }
type Named interface{ Name() string }
type Sq struct{ s float64 }

func (s Sq) Area() float64 { return s.s * s.s }

func main() {
	var sh Shape = Sq{2}
	fmt.Println(sh.Area())
	n := sh.(Named)
	fmt.Println(n.Name())
}
