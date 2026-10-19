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

func twice!(s string) (n int, err error) {
	defer errors.Wrapf(&err, "twice %s", s)
	n = num(s)
	return n * 2, nil
}

func main() {
	fmt.Println(twice("4"))
	n, err := twice("x")
	fmt.Println(n, err, errors.Is(err, ErrBad))
}

// Output:
// 8 <nil>
// 0 twice x: num "x": bad true
