// phase: 12
// error: say is printf-like

// A Go string at the format place of a printf-like function would be read as a format, so a % in
// its text is a conversion: the call is an error, and the message names the two ways out.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	text := "50%" + fmt.Sprint(" done")
	fmt.Println(adapt.say(text))
}
