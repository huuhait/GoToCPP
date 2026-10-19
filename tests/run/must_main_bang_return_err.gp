// phase: 10
// ExitCode: 1
// Stderr: (?m)^[^\n]*: too big: 7$

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

func main!() {
	defer fmt.Println("main deferred")
	n := num("7")
	if n > 5 {
		return fmt.Errorf("too big: %d", n)
	}
	fmt.Println("small")
}

// Output:
// main deferred
