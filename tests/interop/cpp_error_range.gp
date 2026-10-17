// phase: 04

// Errors of range over C/C++ values (step 4.7) that the C++ compiler finds, at the range's line:
// the rows of a C array of arrays can't be copied into the loop variable (Go would copy each
// row; the index form works), a C number is no range (convert it), and a C++ value without
// begin() and end() isn't either.
package main

import (
	"./crange.h"
	"<string>"
)

func main() {
	for _, row := range crange.grid { // ERROR "C\+\+: gp::range: the elements are C arrays, which can't be copied into the loop variable; use gp::range_keys\(c\) and c\[i\]"
		_ = row
	}
	for i := range crange.grid { // fine
		_ = crange.grid[i][0]
	}
	n := C.int(3)
	for i := range n { // ERROR "C\+\+: gp::range: range over a C number: convert it to a gocpp integer, int\(n\)"
		_ = i
	}
	p := crange.make_box()
	for range p { // ERROR "C\+\+: gp::range: range over a C/C\+\+ value without begin\(\) and end\(\): it works over C\+\+ containers, views and C arrays"
	}
}
