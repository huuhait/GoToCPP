// phase: 04

// <string.h> (a package named string is a Go name, so it is imported under a name): strlen,
// strcmp, strchr, memcpy, memset on C buffers.
package main

import (
	"fmt"
	"unsafe"

	cstring "<string.h>"
)

func main() {
	fmt.Println(int(cstring.strlen("hello")), int(cstring.strcmp("a", "b")) < 0, int(cstring.strcmp("b", "b")))
	s := C.CString("find the x here")
	defer C.free(unsafe.Pointer(s))
	x := cstring.strchr(s, 'x')
	fmt.Println(C.GoString(x))

	var a, b [8]C.char
	cstring.memset(unsafe.Pointer(&a[0]), 'z', 7)
	cstring.memcpy(unsafe.Pointer(&b[0]), unsafe.Pointer(&a[0]), 8)
	fmt.Println(C.GoString(&b[0]), int(cstring.strlen(&b[0])))
	fmt.Println(cstring.strstr(s, "nope") == nil)
}

// Output:
// 5 true 0
// x here
// zzzzzzz 7
// true
