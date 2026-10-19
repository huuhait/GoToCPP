// phase: 09

// assert_const (variant hand): the same check written as an if.
package main

import (
	"fmt"
	"os"
)

func run(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += i & 0xff
		if !(i >= 0) {
			panic(fmt.Sprintf("%s:%d: %s: %s", "main.gp", 20, "i >= 0", "i is negative"))
		}
	}
	return sum
}

func main() {
	fmt.Println(run(300_000_000 + len(os.Args) - 1))
}
