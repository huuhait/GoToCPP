// phase: 04

package main

import (
	"fmt"
	"unsafe"

	"./cookbook_callbacks.h"
	"./cookbook_memory.h"
	"<memory>"
	"<stdlib.h>"
	"<vector>"
)

// count adds up the indexes that each() passes, through the void* that C hands back.
func count(user unsafe.Pointer, i C.int) {
	*(*int)(user) += int(i)
}

func main() {
	// 1. Memory from C stays C's: a raw pointer, valid until C frees it. Go does not free it for you.
	head := cookbook_memory.list_make(3) // *cookbook_memory.node, owned by C
	fmt.Println(int(cookbook_memory.list_sum(head)), int(head.next.value))
	cookbook_memory.list_free(head) // head and its nodes are dangling now: do not use them

	// 2. A C array becomes a Go slice that is a view, not a copy; C still owns the memory.
	p := cookbook_memory.ints_make(4)
	squares := unsafe.Slice(p, 4)
	fmt.Println(int(squares[3]))
	stdlib.free(unsafe.Pointer(p))

	// 3. A string that C keeps must live in C memory: C.CString copies it, and you free it.
	name := C.CString("kept by C")
	fmt.Println(C.GoString(name))
	C.free(unsafe.Pointer(name))

	// 4. &T{} of a C type is a Go pointer that owns the new object (like Go's &T{}): it lives as
	//    long as something points to it, and a store into a C struct's member keeps it alive too.
	var list *cookbook_memory.node
	for i := 3; i > 0; i-- {
		list = &cookbook_memory.node{value: C.int(i), next: list} // each node keeps the next one
	}
	fmt.Println(int(cookbook_memory.list_sum(list)))

	// 5. A C++ object that a closure or a defer uses lives in a shared box: the closure and the
	//    function see the one vector, and it is alive when the deferred call runs.
	v := std.vector[int]{}
	add := func(x int) { v.push_back(x) }
	add(1)
	add(2)
	defer fmt.Println("deferred sees", int(v.size()))

	// 6. A smart pointer is a C++ object: dereference it yourself to reach the pointee.
	sp := std.make_shared[std.vector[int]](3, 7)
	fmt.Println(int((*sp).size()), int((*sp)[0]))

	// 7. A Go pointer that goes to C is borrowed: C may use it during the call, never keep it.
	total := 0
	cookbook_callbacks.each(4, count, unsafe.Pointer(&total))
	fmt.Println(total)
}

// Output:
// 6 2
// 9
// kept by C
// 6
// 3 7
// 6
// deferred sees 2
