// phase: 09
// Stderr: ^Usage of tool:\n  -v\tverbose\n$

// -h with ExitOnError prints the usage to os.Stderr and exits with code 0.

package main

import (
	"flag"
	"fmt"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	fs.Bool("v", false, "verbose")
	fmt.Println("before")
	fs.Parse([]string{"-h"})
	fmt.Println("not reached")
}

// Output:
// before
