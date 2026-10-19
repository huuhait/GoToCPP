package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

type Num interface{ ~int | ~int64 }

func conv[T Num](s string) (T, error) {
	if len(s) != 1 || s[0] < '0' || s[0] > '9' {
		return 0, fmt.Errorf("conv %q: %w", s, ErrBad)
	}
	return T(s[0] - '0'), nil
}

func both(a, b string) (int, error) {
	x, err := conv[int](a)
	if err != nil {
		return 0, err
	}
	y, err := conv[int64](b)
	if err != nil {
		return 0, err
	}
	return x + int(y), nil
}

func main() {
	fmt.Println(both("3", "4"))
	fmt.Println(both("3", "x"))
	fmt.Println(both("x", "4"))
}
