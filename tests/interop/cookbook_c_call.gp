// phase: 04

package main

import (
	"fmt"

	"<math.h>"
	"<stdio.h>"
	cstring "<string.h>" // "string" is a Go name, so this header needs a name of your choosing
)

func main() {
	// A Go string constant is a C string literal: no conversion.
	stdio.printf("%s has %d letters\n", "gocpp", 5)

	// A Go string variable: C.cstr gives a const char* that lives until the end of the statement.
	name := "interop"
	stdio.printf("hello, %s\n", C.cstr(name))

	// C numbers are C values: convert them to use them as Go numbers.
	n := int(cstring.strlen(C.cstr(name)))
	fmt.Println(n+1, float64(math.sqrt(2)))

	// A char* that C returns is no string until C.GoString copies it.
	buf := make([]C.char, 32)
	stdio.snprintf(&buf[0], 32, "%03d|%s", 7, "ok")
	fmt.Println(C.GoString(&buf[0]))
}

// Output:
// gocpp has 5 letters
// hello, interop
// 8 1.4142135623730951
// 007|ok
