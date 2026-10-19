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

func f(s string) (int, string, bool, error) {
	n, err := num(s)
	if err != nil {
		return 0, "", false, err
	}
	k, t, b, err := triple(s + s)
	if err != nil {
		return 0, "", false, err
	}
	return n + k, t, b, nil
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}
