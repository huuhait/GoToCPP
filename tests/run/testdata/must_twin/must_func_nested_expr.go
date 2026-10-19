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


func calc(a, b string) (int, error) {
	x, err := num(a)
	if err != nil {
		return 0, err
	}
	y, err := num(b)
	if err != nil {
		return 0, err
	}
	return x*10 + y, nil
}

func main() {
	fmt.Println(calc("4", "2"))
	_, err := calc("4", "x")
	fmt.Println(err)
	_, err = calc("y", "x")
	fmt.Println(err)
}
