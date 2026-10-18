// phase: 04

// Slices and pointers where C/C++ takes spans and raw pointers (step 4.10, README §2.7): a gocpp
// slice converts implicitly to std::span<T> (the len elements; a sub-slice and the result of
// unsafe.Slice too), a string to std::string_view, and a gocpp pointer to the raw T* that C
// takes (borrowed for the call: nil is NULL). Through a C function's `...` a gocpp pointer would
// be a class object, so it is passed as its raw address (p.gp_get()).
package main

import (
	"fmt"
	"unsafe"

	stdio "<stdio.h>"

	"./umem.hpp"
)

func main() {
	// std::span from slices.
	xs := []C.int{1, 2, 3, 4}
	var none []C.int
	fmt.Println(int(umem.span_sum(xs)), int(umem.span_sum(xs[1:3])), int(umem.span_sum(none)))
	umem.span_fill(xs[1:3], 9)
	fmt.Println(xs)
	bs := []byte("hello")
	fmt.Println(int(umem.span_len(bs)), int(umem.span_first(bs)), int(umem.span_first(bs[:0])), int(umem.span_len(bs[2:])))
	var empty []byte
	fmt.Println(int(umem.span_len(empty)), int(umem.span_first(empty)))
	view := unsafe.Slice(unsafe.SliceData(bs), 3)
	fmt.Println(int(umem.span_len(view)), int(umem.view_len("string view")), int(umem.view_len(string(bs))))

	// A gocpp pointer where C takes T*.
	var n int32 = 1
	umem.set_int((*C.int)(unsafe.Pointer(&n)), 5)
	fmt.Println(n)
	var l int64 = 2
	umem.set_long((*C.long)(unsafe.Pointer(&l)), 6)
	fmt.Println(l)
	arr := [3]int32{1, 2, 3}
	fmt.Println(int(umem.sum_ints((*C.int)(unsafe.Pointer(&arr[0])), 3)))
	fmt.Println(int(umem.blob_len(unsafe.Pointer(&arr[0]), 12)), int(umem.blob_len(nil, 12)))
	var np unsafe.Pointer
	fmt.Println(int(umem.blob_len(np, 7)))

	// Through `...`: sscanf stores into gocpp variables by their addresses, snprintf formats
	// with a C string and a number.
	var a, b int32
	got := stdio.sscanf(C.cstr("12 34"), C.cstr("%d %d"), &a, &b)
	fmt.Println(int(got), a, b)
	var f float64
	stdio.sscanf(C.cstr("2.5"), C.cstr("%lf"), &f)
	fmt.Println(f)
	buf := make([]byte, 32)
	k := stdio.snprintf((*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), C.cstr("%s-%d"), C.cstr("id"), C.int(7))
	fmt.Println(int(k), string(buf[:k]))

	// An owning pointer (&xs[i], &T{...}) through `...` too: its raw address.
	cs := []C.int{1, 2, 3}
	px := &cs[1]
	stdio.sscanf(C.cstr("9"), C.cstr("%d"), px)
	upt := &umem.upoint{}
	stdio.sscanf(C.cstr("4 5"), C.cstr("%d %d"), &upt.x, &upt.y)
	fmt.Println(cs, int(upt.x), int(upt.y))
}

// Output:
// 10 5 0
// [1 9 9 4]
// 5 104 -1 3
// 0 -1
// 3 11 5
// 5
// 6
// 6
// 12 0
// 0
// 2 12 34
// 2.5
// 4 id-7
// [1 9 3] 4 5
