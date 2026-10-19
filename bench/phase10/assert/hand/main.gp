// phase: 09

// assert (variant hand): the same check written as an if.
package main

import (
	"fmt"
	"os"
)

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += i & 0xff
		if !(sum >= 0) {
			panic(fmt.Sprintf("%s:%d: %s: %s", "main.gp", 20, "sum >= 0", fmt.Sprint("sum ", sum, " at ", i)))
		}
	}
	return sum
}

func main() {
	fmt.Println(run(300_000_000 + len(os.Args) - 1))
}
