// phase: 09
// ExitCode: 2
// Stderr: (?m)^panic: .*strconv.Atoi: parsing "80x": invalid syntax

// A must call in the initializer of a package-level variable of an imported package (step 10.13): the
// error becomes a panic before main starts. The variable that it depends on is initialized first.
// The twin (twin/) panics by hand.

package main

import (
	"fmt"

	"example.com/varfail/lib"
)

func main() {
	fmt.Println("main must not run", lib.Port)
}

// Output:
// lib: text
