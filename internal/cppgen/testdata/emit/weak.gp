// Weak pointers (Phase 03 step 3.18, internal/lower/weak.go): weak.Pointer[T] as gp::weak_ptr<T>,
// its zero value and literal, weak.Make (also with its type argument, and of nil), Value (also
// through a pointer, as a method value and as a method expression), ==, a weak field, map value
// and key, and fmt's form of a weak pointer.
package main

import (
	"fmt"
	"weak"
)

type Node struct {
	val    int
	parent weak.Pointer[Node]
}

func main() {
	var z weak.Pointer[Node]
	root := &Node{val: 1}
	kid := &Node{val: 2, parent: weak.Make(root)}
	w := weak.Make(root)
	pw := &w
	get := w.Value
	value := weak.Pointer[Node].Value
	fmt.Println(kid.parent.Value().val, pw.Value().val, get().val, value(w).val)
	fmt.Println(z == weak.Pointer[Node]{}, z == weak.Make[Node](nil), w == kid.parent, z.Value() == nil)
	m := map[weak.Pointer[Node]]int{w: 1}
	m[weak.Make(root)]++
	fmt.Println(m[w], len(m), z)
}
