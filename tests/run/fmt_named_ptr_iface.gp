// phase: 09

// A value of a named pointer type (type P *T, type U unsafe.Pointer) in an interface prints as the
// pointer does: &{...} for a pointer to a struct, <nil> for nil; the box of the interface holds the
// pointer, not its target.
package main

import (
	"fmt"
	"unsafe"
)

type Big struct{ A [4]int }
type PB *Big
type PI *int
type UP unsafe.Pointer
type W struct {
	P PB
	Q any
}

func show[T any](v T) { fmt.Println(v) }

func main() {
	b := &Big{}
	b.A[0] = 7
	var a any = PB(b)
	fmt.Println(a)
	fmt.Printf("%v %+v\n", a, a)
	var n any = PB(nil)
	fmt.Println(n)
	fmt.Println([]any{PI(nil), PB(nil)})
	show(PB(nil))
	show(PB(b))
	fmt.Println(a.(PB) == PB(b))
	fmt.Println(W{PB(b), PB(nil)}.Q, W{Q: PB(b)}.P)

	var u UP
	var ua any = u
	fmt.Println(ua)
	fmt.Println([]any{UP(nil)})
	fmt.Println(ua == any(UP(nil)), ua.(UP) == nil)
	var ub any = unsafe.Pointer(nil)
	fmt.Println(ub)
	x := 5
	var c any = UP(&x)
	fmt.Println(fmt.Sprint(c) == fmt.Sprint(&x), fmt.Sprintf("%p", c) == fmt.Sprintf("%p", &x))
	pi := PI(&x)
	var ia any = pi
	fmt.Println(fmt.Sprint(ia) == fmt.Sprint(&x), fmt.Sprintf("%d", ia) == fmt.Sprintf("%d", &x))
}

// Output:
// &{[7 0 0 0]}
// &{[7 0 0 0]} &{A:[7 0 0 0]}
// <nil>
// [<nil> <nil>]
// <nil>
// &{[7 0 0 0]}
// true
// <nil> <nil>
// <nil>
// [<nil>]
// true true
// <nil>
// true true
// true true
