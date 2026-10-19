// phase: 09

// wrap (variant func): the same function with the wrap written as a Go programmer without macros would, a func
// that takes the code as a func value.
package main

import (
	"fmt"
	"os"
)

var tally int

func tallied(body func()) {
	tally++
	body()
}

func work(i int) (r int) {
	tallied(func() {
		r = i*3 + 1
	})
	return r
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
