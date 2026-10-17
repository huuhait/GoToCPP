// phase: 04

// A literal 0 passed to a C++ range factory with a pointer parameter stays a null pointer
// constant: hoisting it into an int variable (to keep the argument alive for the loop) would
// not convert to const char* (step 4.7).
package main

import (
	"fmt"

	"./crange.h"
)

func main() {
	n := 0
	for _, x := range crange.from_cstr(0) {
		n += int(x)
	}
	for _, x := range crange.from_cstr(nil) {
		n += int(x)
	}
	for _, x := range crange.from_cstr("abc") {
		n += int(x)
	}
	for _, x := range crange.from_cstr(1 - 1) { // an untyped constant expression with value 0
		n += int(x)
	}
	for _, x := range crange.from_cstr(0x0) {
		n += int(x)
	}
	fmt.Println(n)
}

// Output:
// 55
