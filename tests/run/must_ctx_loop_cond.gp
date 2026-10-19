// phase: 05

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
	for i := start; i < lim!(i); i++ {
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

// Output:
// lim 0
// body 0
// lim 1
// lim 2
// body 2
// lim 3
// 3 <nil>
// lim 2
// body 2
// lim 3
// 2 <nil>
// lim 4
// 0 lim 4: bad true
