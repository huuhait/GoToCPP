// phase: 12
// error: C\+\+:|cannot use

// The methods are the same: Palette::first takes float rgba[4].
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var palette adapt.Palette
	var shade float64 = 0.25
	fmt.Println(palette.first(&shade))
}
