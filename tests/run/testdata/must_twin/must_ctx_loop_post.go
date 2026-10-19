package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func next(i int) (int, error) {
	fmt.Println("next", i)
	if i == 3 {
		return 0, fmt.Errorf("next %d: %w", i, ErrBad)
	}
	return i + 1, nil
}

func walk(limit int) (int, error) {
	total := 0
	for i := 0; ; {
		if !(i < limit) {
			break
		}
		if i == 1 {
			var err error
			i, err = next(i)
			if err != nil {
				return 0, err
			}
			continue
		}
		fmt.Println("body", i)
		total += i
		var err error
		i, err = next(i)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

func main() {
	fmt.Println(walk(3))
	n, err := walk(9)
	fmt.Println(n, err, errors.Is(err, ErrBad))
}
