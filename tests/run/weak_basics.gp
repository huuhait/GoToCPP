// phase: 03

// weak.Pointer (Go 1.24's package weak, a compiler intrinsic in gocpp: gp::weak_ptr) where Go
// and gocpp agree: the zero value, weak.Make of nil, ==, Value while a strong pointer holds the
// target, weak pointers in fields, maps (as values and as keys), slices and arrays, through a
// pointer, method values and method expressions, an alias and identity conversions, interior
// pointers, a pointer to a package variable, and fmt's form of a weak pointer ({[] <nil>} for
// the zero value, &{[] <nil>} for a pointer to it).
package main

import (
	"fmt"
	"weak"
)

type T struct{ n int }

type Node struct {
	name   string
	parent weak.Pointer[Node]
	kids   []*Node
}

type W = weak.Pointer[T]

var global T

func mk(n int) *T { return &T{n} }

func main() {
	var z weak.Pointer[T]
	fmt.Println(z.Value() == nil, z == weak.Pointer[T]{}, z == weak.Make[T](nil), weak.Make((*T)(nil)) == z)
	fmt.Println(z)
	fmt.Printf("%v %+v %d %s|\n", z, z, z, z)
	fmt.Println(&z) // a pointer to a weak pointer prints as one to a struct
	fmt.Printf("%v %+v\n", &z, &z)
	var zi weak.Pointer[int64]
	fmt.Printf("%v %d\n", zi, 5, zi)

	p := mk(7)
	w := weak.Make(p)
	fmt.Println(w.Value() == p, w.Value().n, w == weak.Make(p), w != z, w == weak.Make(mk(7)))
	w.Value().n++
	fmt.Println(p.n)

	pw := &w
	f := w.Value
	h := weak.Pointer[T].Value
	var a W = w
	fmt.Println(pw.Value().n, f().n, h(w).n, a.Value().n)
	fmt.Println(weak.Pointer[T](w) == w, W(w) == a, W(z) == z) // identity conversions

	root := &Node{name: "root"}
	kid := &Node{name: "kid", parent: weak.Make(root)}
	root.kids = append(root.kids, kid)
	fmt.Println(kid.parent.Value().name, root.parent.Value() == nil, len(root.kids))

	m := map[string]weak.Pointer[T]{"a": weak.Make(p)}
	fmt.Println(m["a"].Value().n, m["b"].Value() == nil)
	s := []weak.Pointer[T]{weak.Make(p), {}}
	arr := [2]weak.Pointer[T]{w}
	fmt.Println(s[0].Value().n, s[1].Value() == nil, arr[0] == w, arr[1] == z)

	keys := map[weak.Pointer[T]]int{}
	keys[w]++
	keys[weak.Make(p)]++
	keys[z]++
	fmt.Println(len(keys), keys[w], keys[z])

	type pair struct{ a, b weak.Pointer[T] }
	x, y := pair{a: w}, pair{a: weak.Make(p)}
	fmt.Println(x == y, x.b == z)

	// Weak pointers to a field and to an element keep their own address.
	field := weak.Make(&p.n)
	sl := []int{1, 2, 3}
	elem := weak.Make(&sl[1])
	*field.Value() += 10
	*elem.Value() *= 7
	fmt.Println(p.n, sl, weak.Make(&sl[1]) == elem, weak.Make(&sl[0]) == elem)

	g := weak.Make(&global)
	g.Value().n = 3
	fmt.Println(global.n)
}


// Output:
// true true true true
// {[] <nil>}
// {[] <nil>} {_:[] u:<nil>} {[] 0} {[] %!s(unsafe.Pointer=<nil>)}|
// &{[] <nil>}
// &{[] <nil>} &{_:[] u:<nil>}
// {[] <nil>} 5
// %!(EXTRA weak.Pointer[int64]={[] <nil>})true 7 true true false
// 8
// 8 8 8 8
// true true true
// root true 1
// 8 true
// 8 true true true
// 2 2 1
// true true
// 18 [1 14 3] true false
// 3
