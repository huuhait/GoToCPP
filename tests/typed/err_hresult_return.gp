// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot use hresult\.status_of\(2\) \(value of type error\) as C\.long value in return statement\n\tthe call returns an HRESULT, which is an error here

// The error of a call does not go out of a function that returns a C number: C.long(call) is the code.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func code() C.long {
	return hresult.status_of(2)
}

func main() {
	fmt.Println(code())
}
