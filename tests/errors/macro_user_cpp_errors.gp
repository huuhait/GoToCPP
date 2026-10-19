// phase: 05

// A lazy parameter can not have a C or C++ type (step 10.9).

package main

import (
	"<string>"
)

macro LC(f lazy C.int) {} // ERROR "lazy parameter f: a C or C\+\+ type"

macro LS(f lazy std.string) {} // ERROR "lazy parameter f: a C or C\+\+ type"

func main() {}
