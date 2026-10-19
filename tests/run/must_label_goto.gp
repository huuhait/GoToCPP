// phase: 10

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
	x := get!(7)
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
M:
	for i := get!(n); i < 6; i++ {
		seen++
		if i == 1 {
			continue M
		}
		if i == 2 && n < 2 {
			n++
			goto M
		}
	}
	return n*100 + seen, nil
}

func main() {
	fmt.Println(f())
	calls = 0
	fmt.Println(g())
}

// Output:
// get 7 1
// get 7 2
// get 7 3
// 6 <nil>
// get 0 1
// get 1 2
// get 2 3
// 206 <nil>
