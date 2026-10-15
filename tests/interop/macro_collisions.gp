// phase: 04

// Macros of the headers must not break the names that gocpp writes (Phase 04 README §2.3).
// The gocpp type Bool and the variable LIMIT get a _ in C++ (Bool_, LIMIT_), so the header's
// macros Bool and LIMIT keep working for the code that uses them (xmacros.AS_BOOL,
// xmacros.LIMIT); the runtime's Println, which the program can't rename, is #undef'd after the
// includes. Macros are written unqualified: cerrno.errno is errno.
package main

import (
	"fmt"

	"./xmacros.h"
	"<cerrno>"
	"<cstdlib>"
)

type Bool bool

func main() {
	var b Bool = true
	LIMIT := 3
	fmt.Println(b, LIMIT, int(xmacros.LIMIT), int(xmacros.TWICE(21)), int(xmacros.AS_BOOL(7)))
	cstdlib.strtol("99999999999999999999", nil, 10)
	fmt.Println(int(cerrno.errno) == int(cerrno.ERANGE))
}

// Output:
// true 3 10 42 7
// true
