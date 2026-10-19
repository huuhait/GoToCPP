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

// ok fails for n < 0, with a trace.
func ok(n int) (int, error) {
	fmt.Println("ok", n)
	if n < 0 {
		return 0, fmt.Errorf("ok %d: %w", n, ErrBad)
	}
	return n * 2, nil
}

func show(a, b int) int {
	fmt.Println("show", a, b)
	return a + b
}

func f(x string) (int, error) {
	h := say("h")
	n, err := num(x)
	if err != nil {
		return 0, err
	}
	k, err := ok(n)
	if err != nil {
		return 0, err
	}
	return show(h, k), nil
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("q"))
	_, err := f("0")
	fmt.Println(err == nil)
}
