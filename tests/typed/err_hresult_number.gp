// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot use hresult\.status_of\(2\) \(value of interface type error\) as int32 value in variable declaration\n\tthe call returns an HRESULT, which is an error here

// An error is not a number: int32(call) takes the code of the HRESULT.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	var code int32 = hresult.status_of(2)
	fmt.Println(code)
}
