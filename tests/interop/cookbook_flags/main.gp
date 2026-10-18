// phase: 04

// Flags are directives, comments before the package clause. They apply to the whole package:
// the compile of the generated code, your own .c and .cpp files, and the probes that read the headers.
//
//gocpp:cflags -DSCALE=10
//gocpp:link m
package main

import (
	"fmt"

	"./scale.h"
)

func main() {
	fmt.Println(int(scale.scaled(27))) // 10 * cbrt(27)
}

// Output:
// 30
