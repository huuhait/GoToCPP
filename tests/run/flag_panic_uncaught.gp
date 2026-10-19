// phase: 09
// ExitCode: 2
// Stderr: ^flag provided but not defined: -zz\nUsage of tool:\npanic: flag provided but not defined: -zz

package main

import (
	"flag"
	"fmt"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.PanicOnError)
	fs.Parse([]string{"-zz"})
	fmt.Println("not reached")
}

// Output:
