// phase: 03

// weak.Pointer[T] is a compiler intrinsic (gp::weak_ptr) until generics come in Phase 07: the
// type, its aliases, weak.Make and Value are fine; a type defined over it is not yet. println
// takes no weak pointer, a struct for Go ("illegal types for operand: print").
package main

import "weak"

type T struct{}

type WP weak.Pointer[T] // ERROR "a type defined as weak.Pointer\[T\] is not supported yet \(Phase 07\)"

type Alias = weak.Pointer[T]

func main() {
	var a Alias
	w := weak.Make(&T{})
	println(a == w, w.Value() == nil, weak.Make[T](nil) == a)
	println(w) // ERROR "illegal types for operand: println w \(weak.Pointer\[T\]\): println doesn't take arrays or structs"
	_ = WP{}
}
