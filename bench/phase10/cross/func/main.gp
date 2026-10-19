// phase: 09

// cross (variant func): the same with functions of util that take func values.
package main

import (
	"fmt"
	"os"

	"example.com/bench/util"
)

func work(i int) (r int) {
	util.Tally(func() {
		r = i*3 + 1
	})
	return r
}

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += work(i) ^ i
		util.Assert("main.gp", 22, sum >= 0, "sum >= 0", func() string { return fmt.Sprint("sum ", sum, " at ", i) })
	}
	return sum
}

func main() {
	fmt.Println(run(200_000_000+len(os.Args)-1), util.Count)
}
