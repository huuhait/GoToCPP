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

// step prints its name and fails for "bad".
func step(name string) error {
	fmt.Println("step", name)
	if name == "bad" {
		return fmt.Errorf("step %s: %w", name, ErrBad)
	}
	return nil
}

func f(s string) (int, error) {
	_ = step(s)
	n, err := num(s)
	if err != nil {
		fmt.Println("taken:", err)
		return -1, nil
	}
	return n, nil
}

func main() {
	fmt.Println(f("bad"))
	fmt.Println(f("2"))
}
