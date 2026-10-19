// phase: 09

// wrap (variant macro): a wrap macro on a small function called in a hot loop.
package main

import (
	"fmt"
	"os"
)

var tally int

macro Tally() {
	tally++
	#body()
}

#Tally()
func work(i int) int {
	return i*3 + 1
}

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += work(i) ^ i
	}
	return sum
}

func main() {
	fmt.Println(run(300_000_000+len(os.Args)-1), tally)
}
