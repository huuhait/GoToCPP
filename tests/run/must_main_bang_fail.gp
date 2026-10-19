// phase: 10
// ExitCode: 1
// Stderr: (?m)^[^\n]*: step bad: bad$

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// num parses one digit.
func num(s string) (int, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), nil
	}
	return 0, fmt.Errorf("num %q: %w", s, ErrBad)
}

// step prints its name and fails for "bad".
func step(name string) error {
	fmt.Println("step", name)
	if name == "bad" {
		return fmt.Errorf("step %s: %w", name, ErrBad)
	}
	return nil
}

func main!() {
	defer fmt.Println("deferred")
	step("a")
	step("bad")
	fmt.Println("not reached")
}

// Output:
// step a
// step bad
// deferred
