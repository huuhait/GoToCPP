// phase: 05
// ExitCode: 1
// Stderr: (?m)^[^\n]*: check first: boot failed$

// An init! function of an imported package that returns an error (step 10.13): the program ends with
// exit code 1 before main, after the init functions of the packages that came before. The twin
// (twin/) writes the end of init! by hand.

package main

import (
	"fmt"

	"example.com/initfail/boot"
)

func main() {
	fmt.Println("main must not run", boot.ErrBoot)
}

// Output:
// first: var
// first: init
// boot: init after first
