// phase: 04

// A switch on a C/C++ value is an if/else chain with C++'s == (gp::cgo::case_eq), so macros
// (cstdio.EOF) and enum constants (tags.MODE_B) work as case values.
package main

import (
	"fmt"

	"./tags.h"
	"<cstdio>"
)

func kind(c C.int) string {
	switch c {
	case cstdio.EOF:
		return "eof"
	case 'a', 'b':
		return "a or b"
	case tags.MODE_B:
		return "mode b"
	}
	return "other"
}

func main() {
	var m tags.enum_mode = tags.MODE_B
	switch m {
	case tags.MODE_A:
		fmt.Println("A")
	case tags.MODE_B:
		fmt.Println("B")
	}
	fmt.Println(kind(-1), kind('b'), kind(1), kind(7))
}

// Output:
// B
// eof a or b mode b other
