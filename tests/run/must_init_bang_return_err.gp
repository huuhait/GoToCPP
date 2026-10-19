// phase: 05
// ExitCode: 1
// Stderr: (?m)^[^\n]*: init says no$

package main

import (
	"errors"
	"fmt"
)

func init!() {
	fmt.Println("init 1")
}

func init!() {
	fmt.Println("init 2")
	return errors.New("init says no")
}

func main() {
	fmt.Println("main must not run")
}

// Output:
// init 1
// init 2
