// phase: 05

// Embedded fields (the gate opens them in Phase 05; the emitter has their fields since step
// 3.8): a promoted field is a path through the embedded fields (o.A is o.Inner.A), also through
// embedded pointers, and &o.A shares the owner of what it points into. An embedded field is
// named after its type, and fmt prints it as a field.
package main

import "fmt"

type Inner struct {
	A int
	b string
}

type Deep struct{ D int }

type Mid struct {
	*Deep
	M int
}

type Outer struct {
	Inner
	*Mid
	X int
}

func main() {
	var o Outer
	o.A = 1
	o.b = "x"
	o.Mid = &Mid{Deep: &Deep{D: 4}, M: 2}
	o.M++
	o.D *= 10
	fmt.Println(o.A, o.b, o.M, o.D, o.Inner, o.X)
	p := &o
	p.A += 5
	pa := &p.Inner.A
	*pa *= 2
	pd := &o.D
	*pd = 7
	fmt.Println(o.Inner, o.Mid.Deep.D, o.Deep == p.Mid.Deep)
	fmt.Printf("%v %v\n", o.Inner, Inner{A: 3})
}

// Output:
// 1 x 3 40 {1 x} 0
// {12 x} 7 true
// {12 x} {3 }
