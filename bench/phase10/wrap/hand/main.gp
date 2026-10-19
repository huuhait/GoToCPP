// phase: 09

// wrap (variant hand): the same function with the macro's code written inline.
package main

import (
	"fmt"
	"os"
)

var tally int

func work(i int) int {
	tally++
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
