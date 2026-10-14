// phase: 01

// A namespace rename must name a namespace of the imported headers.
package main

import (
	"<filesystem>"          // ERROR "header imports are not supported yet \(Phase 04\)"
	fsys "::std::filesystm" // ERROR "::std::filesystm is not a namespace of the imported headers" "C\+\+ namespaces are not supported yet \(Phase 04\)"
	cfmt "::fmt"            // ERROR "::fmt is not a namespace of the imported headers"
	fs "::std::filesystem"
)

func main() {
	_ = fsys.path("/")
	_ = cfmt.format
	_ = fs.path("/")
}
