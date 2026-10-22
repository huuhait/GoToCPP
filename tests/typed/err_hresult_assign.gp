// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot use 5 \(constant of type int\) as error value in assignment: int does not implement error \(missing method Error\)\n\tthe call returns an HRESULT, which is an error here

// A variable made from the call is an error: a number is not assigned to it.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	hr := hresult.status_of(2)
	hr = 5
	fmt.Println(hr)
}
