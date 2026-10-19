// phase: 12
// error: consts\.kDim is not constant: its type const Dim is not a number, a character, a boolean or a string literal

// A constexpr object of a class type is a C++ constant, but not one that Go has.
package main

import (
	"fmt"

	"./consts.hpp"
)

const d = consts.kDim

func main() {
	fmt.Println(d)
}
