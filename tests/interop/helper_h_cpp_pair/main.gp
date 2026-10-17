// phase: 04

//gocpp:no-extern-c ./mathlib.h
package main

// A .h header with a .cpp file next to it (step 4.16). gocpp includes a .h file that parses as C
// inside extern "C" for the generated code, but mathlib.cpp includes it as C++, so its definitions
// would get C++ linkage and not link with the calls. //gocpp:no-extern-c ./mathlib.h says that
// this .h file is C++: both sides then agree. (Or give the header guards, as in
// helper_h_cpp_guards, or name it .hpp.)

import (
	"fmt"

	"./mathlib.h"
)

func main() {
	fmt.Println(mathlib.add(1, 2), mathlib.twice(21))
}

// Output:
// 3 42
