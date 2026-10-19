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

func f(a, b string) (n int, err error) {
	defer func() { fmt.Println("defer 1: n =", n, "err =", err) }()
	defer fmt.Println("defer 2")
	n, err = num(a)
	if err != nil {
		return 0, err
	}
	fmt.Println("after a")
	m, err := num(b)
	if err != nil {
		return 0, err
	}
	n += m
	return n, nil
}

func main() {
	fmt.Println(f("1", "2"))
	fmt.Println(f("1", "x"))
	fmt.Println(f("x", "1"))
}
