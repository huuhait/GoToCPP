// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func b1(name string, v bool) (bool, error) {
	fmt.Println("b1", name, v)
	if name == "bad" {
		return false, fmt.Errorf("b1 %s: %w", name, ErrBad)
	}
	return v, nil
}

func make1(name string, v bool) func() error {
	return func() error {
		n := 0
		for i := 0; i < 3 && b1!(name, v); i++ {
			n += i
		}
		fmt.Println("n", n)
		return nil
	}
}

func make2(name string, v bool) func() (int, error) {
	count := 0
	return func() (int, error) {
		for i := 0; i < 3; i += 1 {
			if b1!(name, v) && count < 2 {
				count++
			}
		}
		switch {
		case b1!(name, v):
			count += 10
		}
		return count, nil
	}
}

func main() {
	fmt.Println(make1("a", true)())
	fmt.Println(make1("bad", true)())
	fmt.Println(make2("a", true)())
	fmt.Println(make2("bad", true)())
}

// Output:
// b1 a true
// b1 a true
// b1 a true
// n 3
// <nil>
// b1 bad true
// b1 bad: bad
// b1 a true
// b1 a true
// b1 a true
// b1 a true
// 12 <nil>
// b1 bad true
// 0 b1 bad: bad
