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

func must1(n int, err error) int {
	if err != nil {
		panic(err)
	}
	return n
}

var broken = must1(num("z"))

func main() {
	fmt.Println("main", broken)
}
