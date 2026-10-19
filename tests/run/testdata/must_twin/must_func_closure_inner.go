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

func outer(xs []string) (int, error) {
	total := 0
	add := func(s string) error {
		n, err := num(s)
		if err != nil {
			return err
		}
		total += n
		return nil
	}
	for _, x := range xs {
		if err := add(x); err != nil {
			fmt.Println("closure error:", err)
		}
	}
	n, err := num(xs[0])
	if err != nil {
		return 0, err
	}
	return total + n, nil
}

func main() {
	fmt.Println(outer([]string{"1", "2", "3"}))
	v, err := outer([]string{"1", "q", "3"})
	fmt.Println(v, err, errors.Is(err, ErrBad))
	v, err = outer([]string{"z", "1"})
	fmt.Println(v, err)
}
