// phase: 12
// error: C\+\+:|cannot use

// A parameter written float color[3] takes three numbers: the address of one Go float64 is not
// adapted for it (it would be read out of bounds), so it is the C++ error.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var shade float64 = 0.5
	fmt.Println(adapt.mix_color("x", &shade))
}
