// phase: 09

package main

import (
	"fmt"
	"unsafe"
)

type T struct{ A, B int }

func (t T) String() string { return fmt.Sprint("T", t.A+t.B) }

type PT *T
type PI *int
type UP unsafe.Pointer

var g = T{1, 2}
var arr [3]T
var gi = 7

func get() any {
	t := &T{3, 4}
	return PT(t)
}

func main() {
	arr[1] = T{5, 6}
	fmt.Println(PT(&g), PT(&arr[1]), get())
	var xs []any
	for i := 0; i < 3; i++ {
		xs = append(xs, PT(&T{i, i}))
	}
	fmt.Println(xs...)
	fmt.Printf("%v %+v %d\n", PT(&g), PT(&arr[1]), PT(&g))
	fmt.Println(fmt.Sprint(PI(&gi)) == fmt.Sprint(&gi), fmt.Sprint(UP(&gi)) == fmt.Sprint(unsafe.Pointer(&gi)))
	s := []T{{8, 9}}
	fmt.Println(PT(&s[0]))
}

// Output:
// &T3 &T11 &T7
// &T0 &T2 &T4
// &T3 &T11 &{1 2}
// true true
// &T17
