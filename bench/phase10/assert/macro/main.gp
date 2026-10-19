// phase: 09

// assert (variant macro): a passing #Assert with a lazy message that captures two variables, in a hot loop.
package main

import (
	"fmt"
	"os"
)

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += i & 0xff
		#Assert(sum >= 0, fmt.Sprint("sum ", sum, " at ", i))
	}
	return sum
}

func main() {
	fmt.Println(run(300_000_000 + len(os.Args) - 1))
}
