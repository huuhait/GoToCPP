// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func items(n int) ([]int, error) {
	fmt.Println("items", n)
	if n < 0 {
		return nil, fmt.Errorf("items %d: %w", n, ErrBad)
	}
	return []int{n, n + 1, n + 2}, nil
}

func pairs(n int) (map[string]int, error) {
	fmt.Println("pairs", n)
	return map[string]int{"a": n}, nil
}

func f(n int) ([]int, error) {
	var fs []func() int
	for k, v := range items!(n) {
		fs = append(fs, func() int { return k*10 + v })
	}
	for range items!(n + 1) {
		fs = append(fs, func() int { return -1 })
	}
	for k, v := range pairs!(n) {
		fs = append(fs, func() int { return len(k) + v })
	}
	var out []int
	for _, f := range fs {
		out = append(out, f())
	}
	return out, nil
}

func main() {
	fmt.Println(f(1))
	fmt.Println(f(-1))
}

// Output:
// items 1
// items 2
// pairs 1
// [1 12 23 -1 -1 -1 2] <nil>
// items -1
// [] items -1: bad
