// phase: 09

// A wrap macro and a must call of another package in a hot loop (the hand variant): what the macro
// variant means, written out by hand.
package main

import (
	"fmt"
	"os"

	"example.com/evidence/util"
)

func work(i int) int {
	util.Count++
	return i*3 + 1
}

func run(n int) (int, error) {
	acc := 1
	for i := 0; i < n; i++ {
		step, err := util.Step(i, acc)
		if err != nil {
			return 0, err
		}
		acc = step ^ work(i)
	}
	return acc, nil
}

func main() {
	n := 200_000_000 + len(os.Args) - 1
	fmt.Println(run(n))
	fmt.Println(util.Count)
}
