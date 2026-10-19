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

func f(a, b string) (int, error) {
	t1, err := num(a)
	if err != nil {
		return 0, err
	}
	t2, err := num(b)
	if err != nil {
		return 0, err
	}
	t3, err := ok(t1 - t2)
	if err != nil {
		return 0, err
	}
	return t3 + 1, nil
}

func main() {
	fmt.Println(f("5", "2"))
	fmt.Println(f("2", "5"))
	fmt.Println(f("q", "5"))
}
