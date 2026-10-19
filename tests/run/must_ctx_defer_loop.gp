// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// step prints its name and fails for "bad".
func step(name string) error {
	fmt.Println("step", name)
	if name == "bad" {
		return fmt.Errorf("step %s: %w", name, ErrBad)
	}
	return nil
}

func f(names ...string) error {
	for _, n := range names {
		defer step!(n)
	}
	fmt.Println("body")
	return nil
}

func main() {
	fmt.Println(f("a", "b", "c"))
	fmt.Println(f("a", "bad", "c"))
}

// Output:
// body
// step c
// step b
// step a
// <nil>
// body
// step c
// step bad
// step a
// step bad: bad
