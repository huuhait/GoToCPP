// phase: 12
// error: pointer to unsigned long that it may write

// An unsigned long is 64 bits here, a uint32 cannot hold every value the callee may store: the copy
// back would cut it, so the compiler does not make the copy.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var n uint32 = 3
	fmt.Println(adapt.add_one(&n), n)
}
