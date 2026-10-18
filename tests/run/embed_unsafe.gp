// phase: 05

// Sizes and offsets with embedded fields: an embedded field is a data member at its place,
// never a base class; Offsetof(b.y) of a promoted field is the offset in b, as in Go.
package main

import (
	"fmt"
	"unsafe"
)

type A struct {
	x byte
	y int64
}
type B struct {
	z int32
	A
	w byte
}
type Outer struct {
	b byte
	B
	s int16
}

func main() {
	var b B
	fmt.Println(unsafe.Sizeof(b), unsafe.Alignof(b), unsafe.Offsetof(b.z), unsafe.Offsetof(b.A), unsafe.Offsetof(b.w))
	fmt.Println(unsafe.Offsetof(b.x), unsafe.Offsetof(b.y), unsafe.Sizeof(b.A))
	var c Outer
	fmt.Println(unsafe.Sizeof(c), unsafe.Offsetof(c.B), unsafe.Offsetof(c.z), unsafe.Offsetof(c.A), unsafe.Offsetof(c.y), unsafe.Offsetof(c.s))
	b.y = 9
	px := (*int64)(unsafe.Add(unsafe.Pointer(&b), unsafe.Offsetof(b.y)))
	*px += 1
	fmt.Println(*px, b.A.y)
	c.A.y = 5
	py := (*int64)(unsafe.Add(unsafe.Pointer(&c), unsafe.Offsetof(c.y)))
	fmt.Println(*py)
}


// Output:
// 32 8 0 8 24
// 8 16 16
// 48 8 8 16 24 40
// 10 10
// 5
