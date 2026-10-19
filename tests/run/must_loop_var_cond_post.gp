// phase: 10

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

func next(i int) (int, error) {
	if i > 5 {
		return 0, fmt.Errorf("next %d: %w", i, ErrBad)
	}
	return i + 1, nil
}

func lim(i int) (int, error) {
	if i > 7 {
		return 0, fmt.Errorf("lim %d: %w", i, ErrBad)
	}
	return 4, nil
}

func f(step int) ([]int, error) {
	var fs []func() int
	for i := 0; i < lim!(i); i = next!(i) {
		fs = append(fs, func() int { return i })
		if step > 0 && i == step {
			continue
		}
	}
	for i := 0; i < 3; i = next!(i) {
		fs = append(fs, func() int { return i * 10 })
	}
	var out []int
	for _, f := range fs {
		out = append(out, f())
	}
	return out, nil
}

func main() {
	fmt.Println(f(0))
	fmt.Println(f(2))
}

// Output:
// [0 1 2 3 0 10 20] <nil>
// [0 1 2 3 0 10 20] <nil>
