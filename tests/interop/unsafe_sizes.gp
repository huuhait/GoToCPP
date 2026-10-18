// phase: 04

// unsafe.Sizeof, Alignof and Offsetof of C/C++ values and of types with a C/C++ part are the C++
// compiler's numbers (sizeof, alignof, offsetof), as values: a Go constant can't hold them. The
// operand is not evaluated, as in Go. And a void** out-parameter takes &p of an unsafe.Pointer
// variable p: the function's pointer is p's new value when the call ends (cgo's C.f(&p)).
package main

import (
	"fmt"
	"unsafe"

	"./umem.hpp"
)

type Pair struct {
	tag byte
	pt  umem.upoint
}

var calls int

func next() C.int {
	calls++
	return 1
}

func main() {
	var rec umem.urec
	var pt umem.upoint
	pp := &pt
	n := unsafe.Sizeof(rec)
	fmt.Println(n, unsafe.Sizeof(C.int(0)), unsafe.Sizeof(umem.upoint{}), unsafe.Alignof(rec), unsafe.Alignof(C.double(0)))
	fmt.Println(unsafe.Offsetof(rec.vals), unsafe.Offsetof(rec.pts), unsafe.Offsetof(pt.y), unsafe.Offsetof(pp.y))
	fmt.Println(n == uintptr(umem.sizeof_urec()), unsafe.Sizeof(Pair{}) >= 5, unsafe.Offsetof(Pair{}.pt) >= 1)
	buf := make([]byte, unsafe.Sizeof(C.long(0)))
	fmt.Println(len(buf))
	_ = unsafe.Sizeof(next()) // not evaluated
	fmt.Println(calls)

	// A void** out-parameter.
	var p unsafe.Pointer
	umem.alloc_block(&p, 8)
	bytes := unsafe.Slice((*byte)(p), 8)
	fmt.Println(p != nil, len(bytes), bytes[0], bytes[7])
	C.free(p)
	ps := make([]unsafe.Pointer, 2) // an element of a slice, and of an array
	for i := range ps {
		umem.alloc_block(&ps[i], 4)
	}
	var arr [2]unsafe.Pointer
	umem.alloc_block(&arr[1], 4)
	fmt.Println(*(*byte)(ps[1]), arr[0] == nil, *(*byte)(arr[1]))
	C.free(ps[0])
	C.free(ps[1])
	C.free(arr[1])
	var q unsafe.Pointer = unsafe.Pointer(&n) // a gocpp object: a function that writes nothing leaves it
	umem.no_write(&q)
	fmt.Println(*(*uintptr)(q) == n)
}

// Output:
// 40 4 8 4 8
// 8 24 4 4
// true true true
// 8
// 0
// true 8 122 122
// 122 true 122
// true
