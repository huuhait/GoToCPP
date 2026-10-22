// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot convert 1 \(untyped int constant\) to type interface\{Error\(\) string\}\n\tthe call returns an HRESULT, which is an error here

// A call that returns an HRESULT is an error, and an error has no arithmetic.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	fmt.Println(hresult.status_of(2) + 1)
}
