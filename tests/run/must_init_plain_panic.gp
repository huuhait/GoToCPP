// phase: 05
// ExitCode: 2
// Stderr: (?m)^panic: .*num "q": bad

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

// A plain init has no error result: a failing ! panics, and the program ends before main.
func init() {
	fmt.Println("init start", num!("4"))
	fmt.Println("init", num!("q"))
	fmt.Println("not reached")
}

func main() {
	fmt.Println("main must not run")
}

// Output:
// init start 4
