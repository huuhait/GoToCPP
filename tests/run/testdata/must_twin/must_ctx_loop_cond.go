package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// lim fails once i reaches 4.
func lim(i int) (int, error) {
	fmt.Println("lim", i)
	if i >= 4 {
		return 0, fmt.Errorf("lim %d: %w", i, ErrBad)
	}
	return 3, nil
}

func count(start int) (n int, err error) {
	for i := start; ; i++ {
		l, err := lim(i)
		if err != nil {
			return 0, err
		}
		if !(i < l) {
			break
		}
		n += i
		if i == 1 {
			continue
		}
		fmt.Println("body", i)
	}
	return n, nil
}

func main() {
	fmt.Println(count(0))
	fmt.Println(count(2))
	n, err := count(4)
	fmt.Println(n, err, errors.Is(err, ErrBad))
}
