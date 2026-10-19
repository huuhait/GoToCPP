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

func f(names ...string) (err error) {
	for _, n := range names {
		defer func() {
			if e := step(n); err == nil {
				err = e
			}
		}()
	}
	fmt.Println("body")
	return nil
}

func main() {
	fmt.Println(f("a", "b", "c"))
	fmt.Println(f("a", "bad", "c"))
}
