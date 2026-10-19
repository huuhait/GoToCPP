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

func load(s string) (n int, err error) {
	defer errors.Wrapf(&err, "load %q", s)
	n = num!(s)
	return n, nil
}

func main() {
	fmt.Println(load("5"))
	n, err := load("x")
	fmt.Println(n, err)
	fmt.Println(errors.Is(err, ErrBad), errors.Unwrap(err))
}

// Output:
// 5 <nil>
// 0 load "x": num "x": bad
// true num "x": bad
