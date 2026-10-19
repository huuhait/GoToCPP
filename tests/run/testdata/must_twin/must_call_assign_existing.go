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

func f(s string) (x, y int, err error) {
	x, err = num(s)
	if err != nil {
		return 0, 0, err
	}
	var t1, t2 int
	t1, t2, err = pair(s + ",5")
	if err != nil {
		return 0, 0, err
	}
	x, y = t1, t2
	t3, err := num("1")
	if err != nil {
		return 0, 0, err
	}
	y += t3
	return x, y, nil
}

func main() {
	fmt.Println(f("2"))
	fmt.Println(f("k"))
}
