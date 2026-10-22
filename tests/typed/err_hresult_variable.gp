// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot use hr \(variable of interface type error\) as int32 value in assignment\n\tthe call returns an HRESULT, which is an error here

// The variable is an error too: int32(hr) is no way out of it (the code is int32(call) on the call itself).
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	hr := hresult.status_of(2)
	var x int32
	x = hr
	fmt.Println(x)
}
