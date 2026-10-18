// phase: 05

package main

import (
	"fmt"

	"<stdexcept>"
	"<string>"
	"<vector>"
)

// parse calls a C++ function that throws std::invalid_argument for bad input.
func parse(s string) (n int, err error) {
	// A C++ exception that reaches a function with a defer is a Go panic: recover() catches it,
	// and the value is an error whose text is "C++ exception <type>: <what()>".
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parse %q: %v", s, r)
		}
	}()
	return int(std.stoi(std.string(s))), nil
}

func main() {
	fmt.Println(parse("42"))
	fmt.Println(parse("x"))

	// std::out_of_range from v.at(i) in a function with defers is just as recoverable.
	func() {
		defer func() {
			r := recover()
			_, isErr := r.(error)
			fmt.Printf("recovered a %T, an error: %v\n", r, isErr)
		}()
		v := std.vector[int]{1, 2, 3}
		fmt.Println(int(v.at(5)))
	}()
}

// Output:
// 42 <nil>
// 0 parse "x": C++ exception std::invalid_argument: stoi
// recovered a runtime.CppException, an error: true
