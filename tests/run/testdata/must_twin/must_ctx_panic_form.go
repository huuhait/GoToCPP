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

func mustNum(s string) int {
	n, err := num(s)
	if err != nil {
		panic(err)
	}
	return n * 2
}

func try(s string) {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r)
		if e, ok := r.(error); ok {
			fmt.Println("is bad:", errors.Is(e, ErrBad))
		}
	}()
	fmt.Println(mustNum(s))
}

func main() {
	try("4")
	try("x")
}
