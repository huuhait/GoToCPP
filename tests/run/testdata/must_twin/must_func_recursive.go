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

func sum(xs []string) (int, error) {
	if len(xs) == 0 {
		return 0, nil
	}
	n, err := num(xs[0])
	if err != nil {
		return 0, err
	}
	rest, err := sum(xs[1:])
	if err != nil {
		return 0, err
	}
	return n + rest, nil
}

func main() {
	fmt.Println(sum([]string{"1", "2", "3"}))
	fmt.Println(sum([]string{"1", "x", "3"}))
}
