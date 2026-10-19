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

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func two(a, b string) (int, int, error) {
	say("start")
	x, err := num(a)
	if err != nil {
		return 0, 0, err
	}
	y, err := num(b)
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func main() {
	fmt.Println(two("1", "2"))
	fmt.Println(two("1", "x"))
	fmt.Println(two("y", "x"))
}
