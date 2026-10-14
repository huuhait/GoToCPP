// phase: 05

// A promoted field through an embedded pointer as a target after a target that changes that
// pointer: Go evaluates the implicit pointer before the stores, so the value goes into the old
// pointee.
package main

import "fmt"

type E struct{ F int }

type O struct {
	*E
	G int
}

type D struct{ O }

func main() {
	e0 := &E{}
	o := O{E: e0}
	o.E, o.F = &E{F: 9}, 5
	fmt.Println(e0.F, o.F)
	d := D{O{E: e0}}
	e1 := &E{}
	d.E, d.F = e1, 8
	fmt.Println(e0.F, e1.F)
	po := &O{E: e0}
	po.E, po.F = e1, 7
	fmt.Println(e0.F, e1.F)
	o.F = 1 // a single assignment reads the pointer when it stores
	fmt.Println(o.F, e0.F)
}
