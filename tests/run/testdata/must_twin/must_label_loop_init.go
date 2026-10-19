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

func f(n string) (int, error) {
	total := 0
	t0, err := num(n)
	if err != nil {
		return 0, err
	}
outer:
	for i := t0; i < 6; i++ {
		t1, err := num(n)
		if err != nil {
			return 0, err
		}
		for j := t1; j < 4; j++ {
			if j == 2 {
				continue outer
			}
			if i == 4 {
				break outer
			}
			total += i*10 + j
		}
	}
	return total, nil
}

func main() {
	fmt.Println(f("1"))
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}
