// phase: 04

// A C helper header with a .c file next to it, as a package (step 4.16): this folder is one
// package, run as "gocpp run .", so gocpp compiles words.c with the C compiler. words.h parses as
// C, so gocpp includes it inside extern "C". C.CString makes a malloc'ed copy of a gocpp string
// for C, and C.free frees it, as it frees what C returns, also a void* or nil, or in a defer (a leak
// fails a -asan run).
package main

import (
	"fmt"

	"./words.h"
)

func main() {
	text := "gocpp calls c helpers"
	cs := C.CString(text)
	fmt.Println(words.words_count(cs))
	C.free(cs)

	up := words.words_upper(C.cstr(text)) // C.cstr: a holder, valid to the end of the statement
	fmt.Println(C.GoString(up))
	C.free(up)

	joined := words.words_join("left", C.cstr("right"+"!"))
	s := C.GoString(joined)
	C.free(joined)
	fmt.Println(s, len(s))

	C.free(nil) // does nothing, as in C
	buf := words.words_alloc(64) // a void* from C is freed as it is
	C.free(buf)
	deferred()
}

// deferred frees at the end of the function: a defer of C.free.
func deferred() {
	p := C.CString("freed by defer")
	defer C.free(p)
	fmt.Println(C.GoString(p))
}

// Output:
// 4
// GOCPP CALLS C HELPERS
// left,right! 11
// freed by defer
