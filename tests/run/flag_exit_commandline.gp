// phase: 09
// ExitCode: 2
// Stderr: ^flag needs an argument: -name\nUsage of prog:\n  -count int\n    \thow many \(default 1\)\n  -name string\n    \twho\n$

// flag.Parse reads os.Args[1:] into the command-line flag set (ExitOnError); the usage names os.Args[0].

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	os.Args = []string{"prog", "-count", "5", "-name"}
	flag.Int("count", 1, "how many")
	flag.String("name", "", "who")
	fmt.Println(flag.Parsed())
	flag.Parse()
	fmt.Println("not reached")
}

// Output:
// false
