// phase: 06

// Extern functions across packages (ADR-128): lib declares and exports Twice, which lib/twice.c
// defines; main calls lib.Twice, and declares the same C function itself under another Go name
// (the two declarations are spelled alike, so the program has one function).
package main

import (
	"fmt"

	"example.com/ext/lib"
)

//gocpp:extern C Twice
func twice(x C.int) C.int

func main() {
	fmt.Println(lib.Twice(4), twice(5), lib.Quad(3))
}

// Output:
// 8 10 12
