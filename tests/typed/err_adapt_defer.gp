// phase: 12
// error: cannot be the argument of a deferred or spawned call

// The variable gets the pointer when the call ends; a deferred call ends after the function has
// moved on, so the adaptation is refused there.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var obj *adapt.Obj
	defer adapt.make_object(&obj)
	fmt.Println(obj == nil)
}
