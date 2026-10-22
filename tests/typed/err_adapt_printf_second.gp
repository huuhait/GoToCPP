// phase: 12
// error: say_at is printf-like .*pass it as an argument, adapt.say_at\(1, "%s", text, 7\)

// When the format is not the first argument, the hint keeps the other arguments: the string moves behind
// "%s" and the callee is written as in the program.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	text := "50%" + fmt.Sprint(" done")
	fmt.Println(adapt.say_at(1, text, 7))
}
