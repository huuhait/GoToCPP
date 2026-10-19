// phase: 10

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
	var failed []string
	try := func(n string) {
		if err := step(n); err != nil {
			failed = append(failed, n)
		}
	}
	for _, n := range names {
		try(n)
	}
	step("end")
	fmt.Println("failed:", failed)
}

func main() {
	fmt.Println(run("a", "bad", "b"))
}

// Output:
// step a
// step bad
// step b
// step end
// failed: [bad]
// <nil>
