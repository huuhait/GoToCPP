// phase: 12
// opaque: same

// Numbers by value at a call of a C++ function (typed mode knows the parameter's type):
//   - an integer of any width goes to an integer parameter and wraps like Go's conversion (the
//     C++ conversion is the same since C++20);
//   - a float goes to an integer parameter with Go's conversion, truncating toward zero;
//   - an untyped constant must fit the parameter: Go's rule (the err_adapt_const*.gp programs).
// Opaque mode builds the same program (C++ converts the values that are in range) and prints the same.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var wide int64 = 1<<40 + 5
	var count int = -1
	var small uint8 = 200
	fmt.Println(adapt.take_int(wide), adapt.take_uint(wide), adapt.take_uint(count), adapt.take_char(small))
	fmt.Println(adapt.take_longlong(count), adapt.take_longlong(small), int64(adapt.take_double(wide)))

	var down float64 = 3.75
	var up float32 = -2.5
	fmt.Println(adapt.take_int(down), adapt.take_longlong(up), adapt.take_uchar(down))
	fmt.Println(adapt.take_float(down), adapt.take_double(up))

	// Constants that fit.
	fmt.Println(adapt.take_char(65), adapt.take_char('a'), adapt.take_uchar(255), adapt.take_int(1<<30))
	fmt.Println(adapt.take_uint(1<<32-1), adapt.take_longlong(-1<<62), int64(adapt.take_double(1<<62)), adapt.take_int(3.0))
}

// Output:
// 5 5 4294967295 -56
// -1 200 1099511627781
// 3 -2 3
// 3.75 -2.5
// 65 97 255 1073741824
// 4294967295 -4611686018427387904 4611686018427387904 3
