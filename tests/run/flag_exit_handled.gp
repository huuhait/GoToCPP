// phase: 09
// ExitCode: 123

// A program that defines -h itself is not given the help: Parse returns, the program exits with its own code.

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	var h bool
	fs.BoolVar(&h, "h", false, "my own -h")
	fs.Parse([]string{"-h"})
	fmt.Println("h =", h)
	os.Exit(123)
}

// Output:
// h = true
