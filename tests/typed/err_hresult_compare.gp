// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot convert 0 \(untyped int constant\) to type interface\{Error\(\) string\}\n\tthe call returns an HRESULT, which is an error here

// The HRESULT of a call is an error: S_OK is nil, not the number 0. Compare it with nil, or convert the call to a number.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	hr := hresult.status_of(2)
	if hr != 0 {
		fmt.Println("failed")
	}
}
