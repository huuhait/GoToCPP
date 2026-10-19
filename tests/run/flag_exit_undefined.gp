// phase: 09
// ExitCode: 2
// Stderr: ^flag provided but not defined: -undefined\nUsage of tool:\n  -n int\n    \tcount \(default 3\)\n$

// An ExitOnError flag set prints the error and the usage to os.Stderr and exits with code 2.

package main

import (
	"flag"
	"fmt"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	fs.Int("n", 3, "count")
	fmt.Println("before")
	fs.Parse([]string{"-n", "4", "-undefined"})
	fmt.Println("not reached")
}

// Output:
// before
