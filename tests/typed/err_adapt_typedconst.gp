// phase: 12
// error: C\+\+:|cannot use

// Only an untyped constant goes to an enum; a typed one (or a variable) has a Go type that says it
// is not an enumerator.
package main

import (
	"fmt"

	"./adapt.hpp"
)

const mode int = 2

func main() {
	fmt.Println(adapt.mode_value(mode))
}
