// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

var budget int

func get(n int) (int, error) {
	budget++
	if budget > 60 {
		panic("budget")
	}
	fmt.Println("get", n)
	if n > 90 {
		return 0, fmt.Errorf("get %d: %w", n, ErrBad)
	}
	return n, nil
}

func initCond() (int, error) {
	total := 0
L:
	for i := get!(0); i < get!(3); i++ {
		for j := get!(0); j < 3; j++ {
			if j == 1 {
				continue L
			}
			if i == 2 {
				break L
			}
			total += 10*i + j
		}
	}
	return total, nil
}

func postLabel() (int, error) {
	total := 0
	round := 0
P:
	for i := 0; i < 4; i += get!(1) {
		if i == 1 {
			continue P
		}
		if i == 2 && round < 1 {
			round++
			goto P
		}
		total += i
	}
	return total, nil
}

func failIn(n int) (int, error) {
L:
	for i := get!(n); i < 3; i++ {
		continue L
	}
	return 1, nil
}

func main() {
	fmt.Println(initCond())
	fmt.Println(postLabel())
	fmt.Println(failIn(1))
	fmt.Println(failIn(99))
}

// Output:
// get 0
// get 3
// get 0
// get 3
// get 0
// get 3
// get 0
// 10 <nil>
// get 1
// get 1
// get 1
// get 1
// get 1
// get 1
// 5 <nil>
// get 1
// 1 <nil>
// get 99
// 0 get 99: bad
