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

func f(s string) (string, error) {
	n, err := num(s)
	if err != nil {
		return "", err
	}
	v, err := ok(n)
	if err != nil {
		return "", err
	}
	if v > 6 {
		return "a", nil
	} else {
		w, err := ok(v + 1)
		if err != nil {
			return "", err
		}
		if w > 4 {
			return fmt.Sprint("b", v, w), nil
		}
	}
	return "c", nil
}

func main() {
	fmt.Println(f("4"))
	fmt.Println(f("2"))
	fmt.Println(f("0"))
	fmt.Println(f("x"))
}
