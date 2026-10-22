// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot convert 0 \(untyped int constant\) to type interface\{Error\(\) string\}\n\tthe call returns an HRESULT, which is an error here

// A switch on an error has errors and nil for cases, not numbers.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	hr := hresult.status_of(2)
	switch hr {
	case 0:
		fmt.Println("zero")
	}
}
