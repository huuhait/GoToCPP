// phase: 12
// error: C\+\+:|cannot use

// A const array parameter is read three times: the address of one variable is not adapted for it either.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var count int64 = 3
	fmt.Println(adapt.sum_three(&count))
}
