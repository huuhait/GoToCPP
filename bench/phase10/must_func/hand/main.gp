// phase: 09

// must_func (variant hand): the same, with the checks written by hand.
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

func chain(i, acc int) (int, error) {
	a, err := step(i, acc)
	if err != nil {
		return 0, err
	}
	b, err := step(i+1, a)
	if err != nil {
		return 0, err
	}
	c, err := step(i+2, b)
	if err != nil {
		return 0, err
	}
	return c, nil
}

func run(n int) (int, error) {
	acc := 1
	for i := 0; i < n; i++ {
		v, err := chain(i, acc)
		if err != nil {
			return 0, err
		}
		acc = v
	}
	return acc, nil
}

func main() {
	n := 100_000_000 + len(os.Args) - 1
	fmt.Println(run(n))
}
