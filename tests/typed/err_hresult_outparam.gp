// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot convert 0 \(untyped int constant\) to type interface\{Error\(\) string\}\n\tthe call returns an HRESULT, which is an error here

// The same holds for a call that the probe cannot type, because an argument is the address of a Go number variable.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	var value int32
	hr := hresult.create(&value, 1)
	if hr != 0 {
		fmt.Println("failed")
	}
}
