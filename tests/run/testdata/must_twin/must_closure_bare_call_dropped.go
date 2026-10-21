package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// check fails for "bad".
func check(name string) error {
	if name == "bad" {
		return fmt.Errorf("check %s: %w", name, ErrBad)
	}
	return nil
}

// A closure is its own function, not a must function: the bare call check(n) in it drops the
// error, as in Go, while the same bare call in outer returns the error.
func outer(name string) error {
	step := func(n string) {
		check(n)
		fmt.Println("closure goes on", n)
	}
	step(name)
	if err := check(name); err != nil {
		return err
	}
	fmt.Println("outer goes on", name)
	return nil
}

func main() {
	fmt.Println(outer("ok"))
	err := outer("bad")
	fmt.Println(err, errors.Is(err, ErrBad))
}
