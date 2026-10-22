// phase: 12
// error: C\+\+:|cannot use

// A parameter written void* slots[2] is an array of pointers: the address of one pointer variable is
// not adapted for it.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var slot *adapt.Obj
	adapt.fill_slots(&slot)
	fmt.Println(slot == nil)
}
