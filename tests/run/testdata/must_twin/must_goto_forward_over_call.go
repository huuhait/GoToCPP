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

func fwd(i string, skip bool) (n int, err error) {
	if skip {
		goto done
	}
	{
		t, err := num(i)
		if err != nil {
			return 0, err
		}
		n = t
	}
	fmt.Println("computed", n)
	if n > 100 {
		t, err := num(i)
		if err != nil {
			return 0, err
		}
		return t, nil
	}
done:
	return n, nil
}

func main() {
	fmt.Println(fwd("1", false))
	fmt.Println(fwd("1", true))
	fmt.Println(fwd("x", false))
	fmt.Println(fwd("x", true))
}
