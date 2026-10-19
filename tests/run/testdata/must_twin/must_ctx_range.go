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

func words(s string) ([]string, error) {
	if s == "" {
		return nil, fmt.Errorf("words: empty: %w", ErrBad)
	}
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ' ' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out, nil
}

func total(s string) (int, error) {
	sum := 0
	ws, err := words(s)
	if err != nil {
		return 0, err
	}
	for i, w := range ws {
		n, err := num(w)
		if err != nil {
			return 0, err
		}
		sum += n * (i + 1)
	}
	return sum, nil
}

func main() {
	fmt.Println(total("1 2 3"))
	fmt.Println(total(""))
	fmt.Println(total("1 x 3"))
}
