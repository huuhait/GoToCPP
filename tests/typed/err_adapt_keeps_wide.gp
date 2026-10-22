// phase: 12
// error: set_title_static keeps the pointer that it is given .*use C.WString\(title\)

// The keeps rules of the strings (a name with static in it keeps what it is given) apply to the wide
// copy too: a temporary that dies at the end of the statement is not what such a function wants.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	title := "a" + fmt.Sprint(1)
	adapt.set_title_static(title)
}
