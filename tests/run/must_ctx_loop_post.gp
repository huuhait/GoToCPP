// phase: 05

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
	for i := 0; i < limit; i = next!(i) {
		if i == 1 {
			continue
		}
		fmt.Println("body", i)
		total += i
	}
	return total, nil
}

func main() {
	fmt.Println(walk(3))
	n, err := walk(9)
	fmt.Println(n, err, errors.Is(err, ErrBad))
}

// Output:
// body 0
// next 0
// next 1
// body 2
// next 2
// 2 <nil>
// body 0
// next 0
// next 1
// body 2
// next 2
// body 3
// next 3
// 0 next 3: bad true
