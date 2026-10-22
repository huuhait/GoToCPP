// phase: 12
// opaque: reject

// A constant integer goes to an enum parameter as a static_cast, when the value is one that C++
// defines for the enum (an enum class: the range of its type; another enum: the bits that its
// enumerators use). A name for the value is as good as a literal.
package main

import (
	"fmt"

	"./adapt.hpp"
)

const flagMask = 6

func main() {
	fmt.Println(adapt.mode_value(2), adapt.mode_value(0x4), adapt.mode_value(1+2))
	fmt.Println(adapt.mode_value(flagMask), adapt.mode_value(adapt.MODE_C))
	fmt.Println(adapt.level_value(5), adapt.level_value(200), adapt.level_value(adapt.Level.High))
}

// Output:
// 2 4 3
// 6 4
// 5 200 9
