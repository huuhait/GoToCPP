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

func pick(a, b string) (string, error) {
	n, err := num(a)
	if err != nil {
		return "", err
	}
	if n > 5 {
		return "big", nil
	} else {
		m, err := num(b)
		if err != nil {
			return "", err
		}
		k, err := ok(m)
		if err != nil {
			return "", err
		}
		if k > 6 {
			return "second", nil
		} else {
			m2, err := num(b)
			if err != nil {
				return "", err
			}
			k2, err := ok(-m2)
			if err != nil {
				return "", err
			}
			if k2 > 0 {
				return "never", nil
			}
		}
	}
	return "none", nil
}

func main() {
	fmt.Println(pick("7", "x"))
	fmt.Println(pick("1", "4"))
	fmt.Println(pick("1", "1"))
	fmt.Println(pick("1", "x"))
	fmt.Println(pick("q", "1"))
}
