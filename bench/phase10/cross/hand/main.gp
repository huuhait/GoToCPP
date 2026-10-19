// phase: 09

// cross (variant hand): the same, with the macros' code written inline in the package that uses them.
package main

import (
	"fmt"
	"os"

	"example.com/bench/util"
)

func work(i int) int {
	util.Count++
	return i*3 + 1
}

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += work(i) ^ i
		if !(sum >= 0) {
			panic(fmt.Sprintf("%s:%d: %s: %s", "main.gp", 22, "sum >= 0", fmt.Sprint("sum ", sum, " at ", i)))
		}
	}
	return sum
}

func main() {
	fmt.Println(run(200_000_000+len(os.Args)-1), util.Count)
}
