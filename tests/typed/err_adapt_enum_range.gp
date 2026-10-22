// phase: 12
// error: outside the values that C\+\+ defines for the enum

// 300 is not a value of an enum class over unsigned char: the cast would be wrong, so it is an error.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	fmt.Println(adapt.level_value(300))
}
