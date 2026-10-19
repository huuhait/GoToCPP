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

// ok fails for n < 0, with a trace.
func ok(n int) (int, error) {
	fmt.Println("ok", n)
	if n < 0 {
		return 0, fmt.Errorf("ok %d: %w", n, ErrBad)
	}
	return n * 2, nil
}

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func f(s string) (int, error) {
	a0, err := ok(1)
	if err != nil {
		return 0, err
	}
	b0, err := ok(2)
	if err != nil {
		return 0, err
	}
	a, b := a0, b0
	var i, v int
	i0 := say("i")
	v0, err := num(s)
	if err != nil {
		return 0, err
	}
	i, v = i0, v0
	a, b = b, a
	return a*100 + b*10 + i + v, nil
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}
