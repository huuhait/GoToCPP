// phase: 07

// An instance of a generic function as a C callback is a plain C function pointer (qsort's comparator, a callback of a table), and a generic func value with a written C++ type argument is a value.
package main

import (
	"fmt"
	"unsafe"

	"./cbtable.h"
	"<stdlib.h>"
	"<string>"
)

func Cmp[T ~int32 | ~int64](a, b unsafe.Pointer) C.int {
	x, y := *(*T)(a), *(*T)(b)
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

func AddUser[T ~int](user unsafe.Pointer, x C.int) C.int {
	p := (*T)(user)
	*p += T(x)
	return C.int(*p)
}

func Id[T any](x T) T { return x }

func main() {
	xs := []int32{5, 2, 9, 1}
	stdlib.qsort(unsafe.Pointer(&xs[0]), 4, 4, Cmp[int32])
	fmt.Println(xs)
	ys := []int64{7, 3, 8}
	stdlib.qsort(unsafe.Pointer(&ys[0]), 3, 8, Cmp[int64])
	fmt.Println(ys)
	total := 100
	fmt.Println(cbtable.cb_sum(4, AddUser[int], unsafe.Pointer(&total)), total)
	h := Id[std.string]
	fmt.Println(int(h(std.string("four")).size()))
}

// Output:
// [1 2 5 9]
// [3 7 8]
// 410 106
// 4
