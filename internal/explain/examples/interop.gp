package main

import (
	"fmt"
	cstdlib "<stdlib.h>"
	cstring "<string.h>"
)

func main() {
	n := cstring.strlen(C.cstr("hello")) // a C call with a C string
	fmt.Println(n, cstdlib.abs(-3))
}
