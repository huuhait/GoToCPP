// phase: 12
// error: set may keep the pointer that it is given

// The class has a char pointer member: its setter may store the pointer, and a Go string's bytes
// live only until the end of the statement.
package main

import (
	"fmt"

	"./strs.hpp"
)

func main() {
	var k strs.Keeper
	name := "n" + fmt.Sprint(1)
	k.set(name)
}
