// phase: 09

// must_call (variant bang): F! in a loop, one must call per iteration.
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

func run(n int) (int, error) {
	acc := 1
	for i := 0; i < n; i++ {
		acc = step!(i, acc)
	}
	return acc, nil
}

func main() {
	n := 300_000_000 + len(os.Args) - 1
	fmt.Println(run(n))
}
