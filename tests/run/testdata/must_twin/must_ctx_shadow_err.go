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

func f(s string) (int, error) {
	n, err := num(s)
	fmt.Println("first:", n, err)
	m, e1 := num("2")
	if e1 != nil {
		return 0, e1
	}
	k, err := num(s + s)
	fmt.Println("second:", k, err)
	if err != nil {
		err := fmt.Errorf("inner: %w", err)
		fmt.Println(err)
	}
	j, e2 := num(s)
	if e2 != nil {
		return 0, e2
	}
	return n + m + k + j, err
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}
