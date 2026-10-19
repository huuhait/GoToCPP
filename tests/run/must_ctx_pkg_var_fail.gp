// phase: 05
// ExitCode: 2
// Stderr: (?m)^panic: .*num "z": bad

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

var broken = num!("z")

func main() {
	fmt.Println("main", broken)
}

// Output:
