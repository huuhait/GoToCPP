// phase: 12
// opaque: reject

// A printf-like function takes its format from a constant, and a runtime string as an argument of
// %s (the string is a const char* there). The string that is the format itself is an error
// (err_adapt_printf.gp).
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	text := "50%" + fmt.Sprint(" done")
	fmt.Println(adapt.say("%s!", text), adapt.say("[%s|%d]", text, 7))
	fmt.Println(adapt.say("plain"))
}

// Output:
// 9 12
// 5
