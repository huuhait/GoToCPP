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

func grid(rows []string) (int, error) {
	total := 0
outer:
	for i := 0; i < len(rows); i++ {
		for j := 0; j < len(rows[i]); j = j + 1 {
			v, err := num(rows[i][j : j+1])
			if err != nil {
				return 0, err
			}
			if v == 0 {
				continue outer
			}
			if v > 7 {
				break outer
			}
			k, err := ok(v)
			if err != nil {
				return 0, err
			}
			total += k
		}
	}
	return total, nil
}

func main() {
	fmt.Println(grid([]string{"12", "30", "45"}))
	fmt.Println(grid([]string{"12", "8", "45"}))
	fmt.Println(grid([]string{"12", "1x", "45"}))
}
