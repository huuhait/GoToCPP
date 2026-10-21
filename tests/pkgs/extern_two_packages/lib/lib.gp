// Package lib declares an extern function that its own C file defines, and exports it.
package lib

// Twice is defined in twice.c.
//
//gocpp:extern C
func Twice(x C.int) C.int

// Quad uses it from Go.
func Quad(x int) int {
	return int(Twice(Twice(C.int(x))))
}
