// phase: 12
// error: C\+\+:|cannot use

// The same for a method: Rack::total takes IVec.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var rack adapt.Rack
	var count int64 = 3
	fmt.Println(rack.total(&count))
}
