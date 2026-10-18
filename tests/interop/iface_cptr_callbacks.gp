// phase: 05

// Pointers to C values (gp::cptr), gocpp pointers, unsafe.Pointer and gocpp functions that C can
// call (step 4.11's callbacks) stored in interfaces: an interface holds each as it is, a gocpp
// func value is a Go func in it (a func value known only at run time is still no C function
// pointer), and assertions and type switch cases tell them apart.
package main

import (
	"fmt"
	"unsafe"

	"<stdlib.h>"
)

func byValue(a, b unsafe.Pointer) C.int {
	x, y := *(*C.int)(a), *(*C.int)(b)
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

type pair struct{ k, v int }

func main() {
	n := 7
	var a any = &n
	fmt.Println(*a.(*int))
	var cs [4]C.int = [4]C.int{3, 1, 4, 2}
	p := &cs[1]
	var b any = p
	q, ok := b.(*C.int)
	fmt.Println(ok, int(*q))
	var f any = byValue
	g := f.(func(unsafe.Pointer, unsafe.Pointer) C.int)
	fmt.Println(int(g(unsafe.Pointer(&cs[0]), unsafe.Pointer(&cs[1]))))
	var items []any
	var raw any = &cs[2]
	switch v := raw.(type) {
	case *C.int:
		fmt.Println("C pointer", int(*v))
	}
	items = append(items, byValue, &n, pair{1, 2}, unsafe.Pointer(&n))
	cnt := 0
	for _, it := range items {
		switch it.(type) {
		case func(unsafe.Pointer, unsafe.Pointer) C.int:
			cnt += 1
		case *int:
			cnt += 10
		case pair:
			cnt += 100
		case unsafe.Pointer:
			cnt += 1000
		}
	}
	fmt.Println(cnt)
	stdlib.qsort(unsafe.Pointer(&cs[0]), 4, 4, byValue)
	fmt.Println(int(cs[0]), int(cs[3]))
}

// Output:
// 7
// true 1
// 1
// C pointer 4
// 1111
// 1 4
