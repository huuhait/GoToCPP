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

func run(names ...string) error {
	for _, n := range names {
		if err := step(n); err != nil {
			return err
		}
	}
	fmt.Println("all done")
	return nil
}

func main() {
	fmt.Println(run("a", "b"))
	err := run("a", "bad", "c")
	fmt.Println(err, errors.Is(err, ErrBad))
}
