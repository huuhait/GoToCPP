// phase: 04

// Callbacks into a C library with a .c file next to it (step 4.11): C frames compiled without
// -fexceptions between the caller and the callback. The handlers are stored in a C struct,
// built by a keyed literal, and called later by name; their parameters are raw C pointers (a
// *C.char arrives as a pointer that owns nothing).
package main

import (
	"fmt"
	"unsafe"

	"./ctable.h"
)

type tally struct {
	names []string
	bytes int
}

func visit(user unsafe.Pointer, i C.int) C.int {
	t := (*tally)(user)
	t.bytes += int(i)
	return i * i
}

func onOpen(name *C.char) {
	fmt.Println("open", C.GoString(name))
}

func onData(user unsafe.Pointer, data *C.char, n C.int) C.int {
	t := (*tally)(user)
	t.names = append(t.names, C.GoStringN(data, n))
	return C.int(len(t.names))
}

func main() {
	t := &tally{}
	fmt.Println(ctable.ctab_walk(5, visit, unsafe.Pointer(t)), t.bytes, ctable.ctab_cleanups())

	h := ctable.ctab_handlers{on_open: onOpen, on_data: onData, user: unsafe.Pointer(t)}
	ctable.ctab_set_handlers(&h)
	ctable.ctab_open("db")
	r1 := ctable.ctab_data(C.cstr("abcdef"), 3)
	r2 := ctable.ctab_data(C.cstr("xyz"), 2)
	fmt.Println(r1, r2, t.names)
}

// Output:
// 30 10 1
// open db
// 1 2 [abc xy]
