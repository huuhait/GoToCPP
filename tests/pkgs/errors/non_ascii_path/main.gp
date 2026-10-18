// phase: 06

// An import path with a character that is not ASCII is refused, like the go command does
// (module.CheckImportPath): the generated files and the namespace are made of the path.
package main

import (
	"fmt"

	hello "example.com/e5/uni/héllo" // ERROR "malformed import path .*: invalid char 'é'"
)

func main() { fmt.Println(hello.Hi()) }
