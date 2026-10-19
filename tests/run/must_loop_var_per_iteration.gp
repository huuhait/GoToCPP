// phase: 05

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

func get(n int) (int, error) {
	if n < 0 {
		return 0, fmt.Errorf("get %d: %w", n, ErrBad)
	}
	return n, nil
}

func f(start int) ([]int, error) {
	var fs []func() int
	var ps []*int
	for i := get!(start); i < 3; i++ {
		fs = append(fs, func() int { return i })
		ps = append(ps, &i)
	}
	for i, j := get!(0), get!(10); i < 2; i, j = i+1, j+1 {
		fs = append(fs, func() int { return i*100 + j })
	}
	var out []int
	for _, f := range fs {
		out = append(out, f())
	}
	for _, p := range ps {
		out = append(out, *p)
	}
	return out, nil
}

func main() {
	fmt.Println(f(0))
	fmt.Println(f(-1))
}

// Output:
// [0 1 2 10 111 0 1 2] <nil>
// [] get -1: bad
