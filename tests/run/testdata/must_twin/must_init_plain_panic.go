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

// A plain init has no error result: a failing ! panics, and the program ends before main.
func init() {
	fmt.Println("init start", must1(num("4")))
	fmt.Println("init", must1(num("q")))
	fmt.Println("not reached")
}

func main() {
	fmt.Println("main must not run")
}
