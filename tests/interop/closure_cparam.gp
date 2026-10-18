// phase: 04

// A func literal that takes or returns a C/C++ value (step 4.11; it was "not supported yet"
// before): a func variable of that type, called from gocpp, and a closure that uses a C++ object
// of the function around it (closure_members.gp).
package main

import (
	"fmt"

	"./objects.h"
)

func main() {
	var c objects.Counter
	f := func(d C.int) { c.add(d) }
	f(1)
	f(2)
	double := func(d C.int) C.int { return d * 2 }
	var g func(C.int) C.int = double
	fmt.Println(int(c.get()), int(g(21)))
}

// Output:
// 3 42
