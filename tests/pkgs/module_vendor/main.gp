// phase: 06

// Packages of a vendored module (vendor/modules.txt): with a package below it. Vendored packages
// are packages like those of the main module.
package main

import (
	"fmt"

	"example.org/ext"
)

func main() {
	fmt.Println(ext.Shout("hey"))
}

// Output:
// init ext
// HEY!
