// phase: 06

// An import that names no package of the module, and the import of a main package.
package main

import (
	"example.com/e3/cmd"     // ERROR "import .example.com/e3/cmd. is a program, not an importable package"
	"example.com/e3/missing" // ERROR "package example.com/e3/missing is not in module example.com/e3"
)

func main() {}
