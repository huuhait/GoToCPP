// phase: 12
// error: set may keep the pointer that it is given .*use C.WString\(title\)

// The class has a wide character pointer member: its setter may store the pointer, and the wide copy
// of a Go string lives only until the end of the statement.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var window adapt.Window
	title := "t" + fmt.Sprint(1)
	window.set(title)
	fmt.Println(window.length())
}
