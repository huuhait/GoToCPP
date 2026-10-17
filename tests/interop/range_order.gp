// phase: 04

// The arguments of the range expression of a range over a C/C++ value are evaluated in Go's
// order, left to right (step 4.7), and the result of the call, which refers to one of its
// temporary arguments, lives as long as the loop (an ASan run checks it). g++ evaluates the
// arguments of a C++ call right to left, so the order is gocpp's, not the compiler's.
package main

import (
	"fmt"

	"./crange.h"
	"<string>"
)

func main() {
	var p crange.tracer
	for _, x := range p.pick(p.tag(1), p.tr(2), p.tr(3)) {
		fmt.Print(" ", int(x))
	}
	fmt.Println()
	for _, x := range p.pick(p.tag(2), p.tr(5), p.tr(6)) {
		fmt.Print(" ", int(x))
	}
	fmt.Println()
	for _, n := range p.log {
		fmt.Print(" ", int(n))
	}
	fmt.Println()
	// A temporary that the program makes itself is kept for the whole loop (one that C++ makes
	// by an implicit conversion inside the call isn't: docs/spec/interop.md).
	n := 0
	for _, c := range crange.first_ref(std.string("a long string literal, on the heap, not in the small buffer")) {
		n += int(c)
	}
	fmt.Println(n)
}

// Output:
//  2 2 2
//  6 6 6
//  1 2 3 2 5 6
// 5381
