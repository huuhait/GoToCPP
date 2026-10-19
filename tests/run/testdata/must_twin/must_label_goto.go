package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

var calls int

func get(n int) (int, error) {
	calls++
	fmt.Println("get", n, calls)
	if calls > 3 {
		return 0, fmt.Errorf("get %d: %w", n, ErrBad)
	}
	return calls, nil
}

func f() (int, error) {
	total := 0
	tries := 0
L:
	x, err := get(7)
	if err != nil {
		return 0, err
	}
	total += x
	tries++
	if tries < 3 {
		goto L
	}
	return total, nil
}

func g() (int, error) {
	n := 0
	seen := 0
M1:
	t, err := get(n)
	if err != nil {
		return 0, err
	}
M:
	for i := t; i < 6; i++ {
		seen++
		if i == 1 {
			continue M
		}
		if i == 2 && n < 2 {
			n++
			goto M1
		}
	}
	return n*100 + seen, nil
}

func main() {
	fmt.Println(f())
	calls = 0
	fmt.Println(g())
}
