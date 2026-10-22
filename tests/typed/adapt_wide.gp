// phase: 12
// opaque: reject

// A Go string goes to a const wchar_t* or const char16_t* parameter as a wide copy (UTF-16 or
// UTF-32 by the unit's size), alive until the end of the statement, as with const char*. A constant
// string is converted too (a C literal would be a narrow one).
package main

import (
	"fmt"

	"./adapt.hpp"
)

func join(a, b string) string {
	return a + b
}

func main() {
	fmt.Println(adapt.wide_length("héllo"), adapt.wide_length(join("ab", "cd")), adapt.wide_length(""))
	fmt.Println(adapt.unit16_length("a\U0001F600"), adapt.unit16_length(join("x", "yz")))
	name := "café"
	fmt.Println(adapt.wide_length(name[:3]), adapt.wide_length(name))
}

// Output:
// 5 4 0
// 3 3
// 3 4
