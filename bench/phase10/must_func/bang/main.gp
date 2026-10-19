// phase: 09

// must_func (variant bang): a must function with three dropped errors, called with F! from a loop.
package main

import (
	"errors"
	"fmt"
	"os"
)

var errNeg = errors.New("negative")

func step(i, acc int) (int, error) {
	if acc < 0 {
		return 0, errNeg
	}
	return (acc*31 + i) & 0xffffff, nil
}

func chain!(i, acc int) int {
	a := step(i, acc)
	b := step(i+1, a)
	c := step(i+2, b)
	return c
}

func run(n int) (int, error) {
	acc := 1
	for i := 0; i < n; i++ {
		acc = chain!(i, acc)
	}
	return acc, nil
}

func main() {
	n := 100_000_000 + len(os.Args) - 1
	fmt.Println(run(n))
}
