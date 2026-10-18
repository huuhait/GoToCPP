// phase: 04

// <stdlib.h>: strtol with an end pointer, atoi, malloc/free of a C array, qsort with a Go
// function as the comparator, abs, getenv of a missing name (a null pointer).
package main

import (
	"fmt"
	"unsafe"

	"<stdlib.h>"
)

func cmpInt(a, b unsafe.Pointer) C.int {
	x, y := *(*C.int)(a), *(*C.int)(b)
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}

func main() {
	var end *C.char
	s := C.CString("123abc")
	defer C.free(unsafe.Pointer(s))
	v := stdlib.strtol(s, &end, 10)
	fmt.Println(int(v), C.GoString(end), int(stdlib.atoi("77")), int(stdlib.abs(-5)))

	p := (*C.int)(stdlib.malloc(5 * C.size_t(unsafe.Sizeof(C.int(0)))))
	arr := unsafe.Slice(p, 5)
	for i, x := range []int{5, 2, 9, 1, 7} {
		arr[i] = C.int(x)
	}
	stdlib.qsort(unsafe.Pointer(p), 5, C.size_t(unsafe.Sizeof(C.int(0))), cmpInt)
	fmt.Println(int(arr[0]), int(arr[1]), int(arr[2]), int(arr[3]), int(arr[4]))
	stdlib.free(unsafe.Pointer(p))

	fmt.Println(stdlib.getenv("GOCPP_ZOO_SURELY_UNSET_NAME") == nil)
}

// Output:
// 123 abc 77 5
// 1 2 5 7 9
// true
