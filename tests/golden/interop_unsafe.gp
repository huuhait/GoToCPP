// phase: 04

// unsafe and pointers (step 4.10): unsafe.Pointer as the owning gp::ptr<void> and its
// conversions, unsafe.Add, Slice, SliceData, String and StringData over gocpp memory, the same
// functions over C pointers (a raw pointer: a view that owns nothing), the conversions of C
// pointers and functions, &T{...} of a C type, &s.f and &v[i], a C array sliced, and a gocpp
// pointer through a C function's `...` (p.gp_get()), the address of an element of a gocpp slice
// of C elements (gp::cptr), unsafe.Sizeof of a C value, and a void** out-parameter (out_ptr).
package main

import (
	"fmt"
	"unsafe"

	"<stdio.h>"
	"<stdlib.h>"
	cstring "<string.h>"
)

type Builder struct{ buf []byte }

func (b *Builder) String() string {
	return unsafe.String(unsafe.SliceData(b.buf), len(b.buf))
}

func main() {
	x := 7
	p := unsafe.Pointer(&x)
	*(*int)(unsafe.Add(p, 0)) = 8
	nums := []int32{1, 2, 3}
	tail := unsafe.Slice(&nums[1], 2)
	fmt.Println(x, uintptr(p) != 0, tail, *unsafe.StringData("abc"))

	var cp *C.char
	var n C.int = 3
	up := unsafe.Pointer(cp)
	bs := unsafe.Slice((*byte)(up), n)
	s := unsafe.String((*byte)(up), n)
	fmt.Println(len(bs), s, (*C.char)(up) == nil, unsafe.Pointer(cstring.strlen) != nil)

	words := unsafe.Slice(&cp, 2)
	fmt.Println(len(words))
	pt := &stdio.fpos_t{}
	stdio.fgetpos(nil, pt)
	var a [2]stdio.fpos_t
	fmt.Println(&a[1] != nil, pt != nil)
	stdio.sscanf(C.cstr("5"), C.cstr("%d"), &x)

	// Owning pointers (gp::cptr), the sizes of C values, a void** out-parameter.
	cs := []C.int{1, 2}
	pc := &cs[0]
	var out unsafe.Pointer
	stdlib.posix_memalign(&out, 16, 32)
	fmt.Println(unsafe.Sizeof(cs[0]), pc != nil, out != nil)
	stdlib.free(out)
}
