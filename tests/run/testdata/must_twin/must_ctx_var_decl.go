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

// pair splits "a,b" into two digits.
func pair(s string) (int, int, error) {
	if len(s) == 3 && s[1] == ',' {
		a, err := num(s[:1])
		if err != nil {
			return 0, 0, err
		}
		b, err := num(s[2:])
		return a, b, err
	}
	return 0, 0, fmt.Errorf("pair %q: %w", s, ErrBad)
}

func triple(s string) (int, string, bool, error) {
	if s == "" {
		return 0, "", false, fmt.Errorf("triple: %w", ErrBad)
	}
	return len(s), s + s, len(s) > 1, nil
}

func f(s string) (string, error) {
	n, err := num(s)
	if err != nil {
		return "", err
	}
	a, b, err := pair(s + "," + s)
	if err != nil {
		return "", err
	}
	k, t, big, err := triple(s)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(n, a, b, k, t, big), nil
}

func main() {
	fmt.Println(f("4"))
	fmt.Println(f("x"))
}
