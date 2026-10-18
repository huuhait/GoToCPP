// phase: 05

// A C++ exception that reaches a function with defers is a Go panic with a runtime.CppException
// value: recover() returns an error whose Error() text is what an unrecovered one prints.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

func try(n int) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint("recovered: ", r)
		}
	}()
	cxxvals.throw_runtime(C.int(n))
	return "no exception"
}

func tryInt(n int) (msg string) {
	defer func() {
		r := recover()
		msg = fmt.Sprintf("%v", r)
	}()
	cxxvals.throw_int(C.int(n))
	return "no exception"
}

func main() {
	fmt.Println(try(0))
	fmt.Println(try(7))
	fmt.Println(tryInt(0))
	fmt.Println(tryInt(42))
	fmt.Println("done")
}

// Output:
// no exception
// recovered: C++ exception std::runtime_error: bad thing 7
// <nil>
// C++ exception of type int
// done
