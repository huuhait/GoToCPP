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

func f(xs []string) (int, error) {
	total := 0
	for _, x := range xs {
		if x == "-" {
			continue
		}
		{
			v, err := num(x)
			if err != nil {
				return 0, err
			}
			total += v
		}
		n, err := num(x)
		if err != nil {
			return 0, err
		}
		switch {
		case n > 5:
			total += 100
		default:
			v := n * 2
			total += v
		}
	}
	return total, nil
}

func main() {
	fmt.Println(f([]string{"1", "-", "7"}))
	fmt.Println(f([]string{"1", "q", "7"}))
}
