// phase: 05

// unsafe.Offsetof of a promoted field is the offset in the outer struct, but not through an embedded
// pointer (the field is in another object): go/types' error.
package main

import "unsafe"

type A struct{ x, y int }
type P struct {
	*A
	z int
}
type V struct {
	A
	z int
}

func main() {
	var p P
	var v V
	_ = unsafe.Offsetof(p.x) // ERROR "invalid argument: field x is embedded via a pointer in P"
	_ = unsafe.Offsetof(p.z)
	_ = unsafe.Offsetof(v.x)
	_ = unsafe.Offsetof(v.A)
	_ = unsafe.Sizeof(p.A)
}
