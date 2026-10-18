// phase: 04

// A C char array or char pointer compared with a Go string constant compares addresses in C++
// (silently false for an array with g++): an error at the .gp line. Comparing two C pointers, a
// pointer with nil, a std::string with a constant and C.GoString(p) with a constant are fine
// (c_string_compare.gp).
package main

import (
	"fmt"

	"./cunion.h"
	"<stdlib.h>"
)

func main() {
	it := cunion.cu_item(3, 1.5)
	fmt.Println(it.name == "item") // ERROR "C\+\+: invalid operation: comparing a C string \(char\*\) with a Go string compares addresses: use C\.GoString\(x\) == .\.\.\.. \(or strcmp\)"
	fmt.Println(stdlib.getenv("HOME") == "x") // ERROR "C\+\+: invalid operation: comparing a C string"

	// The same check where the comparison is a condition: if, for, a switch without a tag, and a
	// switch on the C value (each seed has its own C type: the compiler reports an instantiation
	// once). && || ! work the same way.
	if cunion.cu_label() == "label" { // ERROR "C\+\+: invalid operation: comparing a C string"
		fmt.Println("same")
	}
	t := cunion.cu_tag()
	for t.tag != "ab" { // ERROR "C\+\+: invalid operation: comparing a C string"
		break
	}
	switch {
	case t.tag2 == "cunion": // ERROR "C\+\+: invalid operation: comparing a C string"
	}
	p := C.CString("x")
	switch p {
	case "x": // ERROR "C\+\+: invalid operation: comparing a C string"
	}
}
