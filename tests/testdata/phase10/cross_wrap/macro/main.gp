// phase: 09

// A wrap macro and a must call of another package in a hot loop (the macro variant): the same program
// as the hand variant, with the macro and the `!` written out there.
package main

import (
	"fmt"
	"os"

	"example.com/evidence/util"
)

#util.Tally()
func work(i int) int {
	return i*3 + 1
}

func run(n int) (int, error) {
	acc := 1
	for i := 0; i < n; i++ {
		acc = util.Step!(i, acc) ^ work(i)
	}
	return acc, nil
}

func main() {
	n := 200_000_000 + len(os.Args) - 1
	fmt.Println(run(n))
	fmt.Println(util.Count)
}
