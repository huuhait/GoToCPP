// phase: 04

// Several values from one C++ value, a pair or a tuple (std.minmax returns a pair of
// references): gp::values copies them, so they don't dangle (an ASan run checks it), also when
// the arguments are temporaries. With :=, =, _ and variables that exist.
package main

import (
	"fmt"

	"<algorithm>"
	"<string>"
	"<tuple>"
	"<utility>"
)

func main() {
	lo, hi := std.minmax(3, 1)
	fmt.Println(int(lo), int(hi))
	x, y := C.int(10), C.int(-4)
	a, b := std.minmax(x, y)
	x = 0 // a and b are copies, not references to x and y
	fmt.Println(int(a), int(b), int(x))
	lo, hi = std.minmax(std.string("pear").size(), std.string("fig").size())
	fmt.Println(int(lo), int(hi))
	_, top := std.minmax(C.int(7), C.int(9))
	n, s, d := std.make_tuple(C.int(1), std.string("two"), 3.5)
	lo, z := std.minmax(C.long(5), C.long(2))
	fmt.Println(int(top), int(n), int(s.size()), float64(d), int(lo), int(z))
}

// Output:
// 1 3
// -4 10 0
// 3 4
// 9 1 3 3.5 2 5
