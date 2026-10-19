// phase: 09
// ExitCode: 2
// Stderr: ^invalid value "x" for flag -n: parse error\nUsage of tool:\n  -n int\n    \tcount\n$

package main

import (
	"flag"
	"fmt"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	fs.Int("n", 0, "count")
	fs.Parse([]string{"-n=x"})
	fmt.Println("not reached")
}

// Output:
