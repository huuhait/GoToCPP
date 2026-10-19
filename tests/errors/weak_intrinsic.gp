// phase: 03

// weak.Pointer[T] is a compiler intrinsic (gp::weak_ptr): the type, its aliases, weak.Make and
// Value are fine; a type defined over it is an error at every phase. println
// takes no weak pointer, a struct for Go ("illegal types for operand: print").
package main

import "weak"

type T struct{}

type WP weak.Pointer[T] // ERROR "a type defined as weak.Pointer\[T\] is not supported in gocpp"

type Alias = weak.Pointer[T]

func main() {
	var a Alias
	w := weak.Make(&T{})
	println(a == w, w.Value() == nil, weak.Make[T](nil) == a)
	println(w) // ERROR "illegal types for operand: println w \(weak.Pointer\[T\]\): println doesn't take arrays or structs"
	_ = WP{}
}
