// phase: 09

package main

import (
	"flag"
	"fmt"
	"testing"
)

func main() {
	// Not in a test binary: Testing is false, and Short and Verbose need Init and flag.Parse (go test's flags).
	testing.Init()
	flag.Parse()
	fmt.Println(testing.Testing(), testing.Short(), testing.Verbose())

	calls := 0
	testing.AllocsPerRun(5, func() { calls++ })
	fmt.Println(calls)
}

// Output:
// false false false
// 6
