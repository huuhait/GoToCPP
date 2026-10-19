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

func add(a, b int) int { return a + b }

func f(a, b string) (int, error) {
	t1 := say(a)
	t2, err := num(a)
	if err != nil {
		return 0, err
	}
	t3 := add(t1, t2)
	t4, err := num(b)
	if err != nil {
		return 0, err
	}
	return add(t3, t4), nil
}

func main() {
	fmt.Println(f("1", "2"))
	fmt.Println(f("1", "z"))
	fmt.Println(f("y", "z"))
}
