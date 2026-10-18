// phase: 04

package main

import (
	"<cstdio>"
	"<cstring>"
)

func main() {
	name := "gocpp"
	cstdio.printf("hello %s (%d letters)\n", C.cstr(name), C.int(len(name)))

	n := cstring.strlen("string literals go straight to C")
	cstdio.printf("strlen = %zu\n", n)
}

// Output:
// hello gocpp (5 letters)
// strlen = 32
