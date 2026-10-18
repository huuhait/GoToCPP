// phase: 01

// A namespace rename must name a namespace of the imported headers.
package main

import (
	"<filesystem>"
	fsys "::std::filesystm" // ERROR "::std::filesystm is not a namespace of the imported headers"
	cfmt "::fmt"            // ERROR "::fmt is not a namespace of the imported headers"
	fs "::std::filesystem"
)

func main() {
	_ = fsys.path("/")
	_ = cfmt.format
	_ = fs.path("/")
}
