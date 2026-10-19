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

func run!(names ...string) {
	for _, n := range names {
		step(n)
	}
	fmt.Println("all done")
}

func main() {
	fmt.Println(run("a", "b"))
	err := run("a", "bad", "c")
	fmt.Println(err, errors.Is(err, ErrBad))
}

// Output:
// step a
// step b
// all done
// <nil>
// step a
// step bad
// step bad: bad true
