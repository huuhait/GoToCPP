// phase: 09
// Stderr: ^custom usage\n$

// -help with ExitOnError calls Usage (here a custom one that prints to os.Stderr) and exits with code 0.

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "custom usage") }
	fs.Bool("v", false, "verbose")
	fs.Parse([]string{"-v", "-help"})
	fmt.Println("not reached")
}

// Output:
