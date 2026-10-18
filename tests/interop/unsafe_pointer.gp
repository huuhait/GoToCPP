// phase: 04

// unsafe.Pointer is an owning pointer for gocpp memory (D16): it keeps the object alive, converts
// to the void* of a C function (borrowed), and goes back to a typed pointer with (*T)(p). Here:
// the conversions of Go's rules, unsafe.Add, uintptr and the address pattern of go vet's rule 3,
// a pointer to a gocpp variable, a field and an element handed to C, and C's malloc memory as
// an unsafe.Pointer.
package main

import (
	"fmt"
	"unsafe"

	"<stdlib.h>"
	cstring "<string.h>"

	"./umem.hpp"
)

type Header struct {
	kind  uint8
	flags uint8
	size  uint32
	owner unsafe.Pointer
}

type Handle unsafe.Pointer

func alloc() unsafe.Pointer {
	x := new(int64) // the pointer is the only owner of the int
	*x = 77
	return unsafe.Pointer(x)
}

func main() {
	// A typed pointer and back: the same object.
	n := 5
	p := unsafe.Pointer(&n)
	*(*int)(p) = 6
	fmt.Println(n, p != nil, p == unsafe.Pointer(&n))

	// The object lives as long as the unsafe.Pointer does.
	q := alloc()
	fmt.Println(*(*int64)(q))
	*(*int64)(q) += 3
	fmt.Println(*(*int64)(q))

	// unsafe.Add and uintptr arithmetic move through an array.
	nums := [4]int32{10, 20, 30, 40}
	base := unsafe.Pointer(&nums[0])
	third := (*int32)(unsafe.Add(base, 2*unsafe.Sizeof(nums[0])))
	fmt.Println(*third, *(*int32)(unsafe.Pointer(uintptr(base) + 4)))
	*third = 33
	fmt.Println(nums)

	// A field of a struct, through the struct's address and offset.
	h := Header{kind: 1, flags: 2, size: 300}
	hp := unsafe.Pointer(&h)
	sizeAt := (*uint32)(unsafe.Add(hp, unsafe.Offsetof(h.size)))
	*sizeAt = 512
	fmt.Println(h.kind, h.flags, h.size)
	h.owner = unsafe.Pointer(&n)
	fmt.Println(h.owner == p, h.owner != nil)

	// Pointers are comparable, usable as map keys, and print as addresses (<nil> for nil).
	var none unsafe.Pointer
	seen := map[unsafe.Pointer]string{p: "n", q: "q"}
	fmt.Println(seen[p], seen[q], len(seen), none == nil, seen[none] == "")
	fmt.Println(none)
	var hd Handle = Handle(p)
	fmt.Println(unsafe.Pointer(hd) == p, uintptr(unsafe.Pointer(hd)) == uintptr(p))

	// Handed to C as a void*: the pointer is borrowed for the call.
	src := []byte("hello, memory")
	dst := make([]byte, len(src))
	cstring.memcpy(unsafe.Pointer(&dst[0]), unsafe.Pointer(&src[0]), C.size_t(len(src)))
	fmt.Println(string(dst))
	cstring.memset(unsafe.Pointer(&dst[0]), 'x', 5)
	fmt.Println(string(dst))
	var a, b int32 = 1, 2
	fmt.Println(cstring.memcmp(unsafe.Pointer(&a), unsafe.Pointer(&b), 4) != 0, cstring.memcmp(unsafe.Pointer(&a), unsafe.Pointer(&a), 4))

	// C's memory as an unsafe.Pointer: a view over it owns nothing, and C frees it.
	m := stdlib.malloc(16)
	mp := unsafe.Pointer(m)
	*(*int32)(mp) = 41
	*(*int32)(unsafe.Add(mp, 4)) = 42
	fmt.Println(*(*int32)(mp)+*(*int32)(unsafe.Add(mp, 4)), mp != nil)
	var v C.int = 9
	umem.set_int((*C.int)(unsafe.Pointer(&v)), 11)
	fmt.Println(int(v))
	C.free(mp)
	C.free(nil)

	// A C function's address is an unsafe.Pointer, and back.
	fp := unsafe.Pointer(umem.triple)
	fn := umem.unary_fn(fp)
	fmt.Println(fp != nil, int(umem.call_unary(fn, 14)), fn == umem.triple, uintptr(fp) != 0)
}

// Output:
// 6 true true
// 77
// 80
// 30 20
// [10 20 33 40]
// 1 2 512
// true true
// n q 2 true true
// <nil>
// true true
// hello, memory
// xxxxx, memory
// true 0
// 83 true
// 11
// true 42 true true
