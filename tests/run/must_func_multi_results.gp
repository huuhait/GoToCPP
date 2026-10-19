// phase: 05

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

func triple(s string) (int, string, bool, error) {
	if s == "" {
		return 0, "", false, fmt.Errorf("triple: %w", ErrBad)
	}
	return len(s), s + s, len(s) > 1, nil
}

func f!(s string) (int, string, bool) {
	n := num(s)
	k, t, b := triple(s + s)
	return n + k, t, b
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}

// Output:
// 5 3333 true <nil>
// 0  false num "x": bad
