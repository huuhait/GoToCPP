// phase: 04

// A slice expression of a C array is a slice, not an addressable value; the message shows the
// slice expression the user wrote.
package main

import "../interop/umem.hpp"

func main() {
	_ = &umem.the_rec.vals[0:1] // ERROR "cannot take address of [A-Za-z.]*the_rec\.vals\[0:1\] \(slice of C/C\+\+ values\)"
}
