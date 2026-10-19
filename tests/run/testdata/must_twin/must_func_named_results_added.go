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

func two(a, b string) (x, y int, err error) {
	x, err = num(a)
	if err != nil {
		return 0, 0, err
	}
	y, err = num(b)
	if err != nil {
		return 0, 0, err
	}
	return
}

func main() {
	fmt.Println(two("1", "2"))
	fmt.Println(two("1", "z"))
	fmt.Println(two("z", "2"))
}
