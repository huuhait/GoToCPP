// phase: 09

// cross (variant macro): macros of another package in a hot loop: a passing #util.Assert with a lazy message
// and a wrap macro #util.Tally on a small function.
package main

import (
	"fmt"
	"os"

	"example.com/bench/util"
)

#util.Tally()
func work(i int) int {
	return i*3 + 1
}

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += work(i) ^ i
		#util.Assert(sum >= 0, fmt.Sprint("sum ", sum, " at ", i))
	}
	return sum
}

func main() {
	fmt.Println(run(200_000_000+len(os.Args)-1), util.Count)
}
