// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot use hresult\.status_of\(2\) \(value of type error\) as C\.long value in variable declaration\n\tthe call returns an HRESULT, which is an error here

// An error does not go into a C number either: C.long(call) takes the code.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	var n C.long = hresult.status_of(2)
	fmt.Println(n)
}
