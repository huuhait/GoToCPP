// phase: 04

// <optional>: has_value, value, value_or, dereference, reset, nullopt (a value: `std.nullopt`),
// and an optional of a std::string.
package main

import (
	"fmt"

	"<optional>"
	"<string>"
)

func find(ok bool) std.optional[int] {
	if ok {
		return std.optional[int](7)
	}
	return std.optional[int]{}
}

func main() {
	a := find(true)
	b := find(false)
	fmt.Println(a.has_value(), b.has_value(), int(a.value()), int(b.value_or(-1)), int(*a))

	var s std.optional[std.string]
	fmt.Println(bool(s.has_value()))
	s = std.string("set")
	fmt.Println(string(*s), int(s.value().size()))
	s.reset()
	s = std.nullopt
	fmt.Println(s.has_value(), s == std.nullopt)
}

// Output:
// true false 7 -1 7
// false
// set 3
// false true
