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
	it, err := items(n)
	if err != nil {
		return nil, err
	}
	for k, v := range it {
		fs = append(fs, func() int { return k*10 + v })
	}
	it2, err := items(n + 1)
	if err != nil {
		return nil, err
	}
	for range it2 {
		fs = append(fs, func() int { return -1 })
	}
	pm, err := pairs(n)
	if err != nil {
		return nil, err
	}
	for k, v := range pm {
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
