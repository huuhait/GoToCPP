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

// pair splits "a,b" into two digits.
func pair(s string) (int, int, error) {
	if len(s) == 3 && s[1] == ',' {
		a, err := num(s[:1])
		if err != nil {
			return 0, 0, err
		}
		b, err := num(s[2:])
		return a, b, err
	}
	return 0, 0, fmt.Errorf("pair %q: %w", s, ErrBad)
}

func f(s string) (int, error) {
	_, err := num(s)
	if err != nil {
		return 0, err
	}
	_, b, err := pair(s + ",1")
	if err != nil {
		return 0, err
	}
	c, err := num("7")
	if err != nil {
		return 0, err
	}
	return b + c, nil
}

func main() {
	fmt.Println(f("2"))
	fmt.Println(f("q"))
}
