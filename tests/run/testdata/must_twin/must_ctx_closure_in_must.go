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
	var failed []string
	try := func(n string) {
		if err := step(n); err != nil {
			failed = append(failed, n)
		}
	}
	for _, n := range names {
		try(n)
	}
	if err := step("end"); err != nil {
		return err
	}
	fmt.Println("failed:", failed)
	return nil
}

func main() {
	fmt.Println(run("a", "bad", "b"))
}
