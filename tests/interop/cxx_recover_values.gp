// phase: 05

// C++ objects that deferred calls use stay valid while a C++ exception unwinds the function (step
// 4.8's boxes), and a result of a C++ type that the function recovers is its zero value.
package main

import (
	"fmt"

	"./cxxvals.hpp"
	"<string>"
	"<vector>"
)

func use() {
	v := std.vector[int]{1, 2, 3}
	s := std.string("text")
	defer func() { fmt.Println("recovered:", recover()) }()
	defer fmt.Println("vector size in a deferred call:", int(v.size()))
	defer v.push_back(4)
	defer func() { fmt.Println("string in a closure:", string(s), int(v.size())) }()
	cxxvals.throw_runtime(2)
}

func label() std.string {
	defer func() { recover() }()
	cxxvals.throw_runtime(1)
	return std.string("not returned")
}

func main() {
	use()
	fmt.Printf("%q\n", string(label()))
}

// Output:
// string in a closure: text 3
// vector size in a deferred call: 3
// recovered: C++ exception std::runtime_error: bad thing 2
// ""
