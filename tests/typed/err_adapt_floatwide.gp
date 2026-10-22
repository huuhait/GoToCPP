// phase: 12
// error: is wider than double|pointer to double that it may write

// A float64 is read through a pointer to a double as it is (the same type); a float32 cannot hold
// every double the callee may store.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var x float32 = 3
	adapt.scale(&x)
	fmt.Println(x)
}
