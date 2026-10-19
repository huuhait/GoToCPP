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
	t0, err := get(0)
	if err != nil {
		return 0, err
	}
L:
	for i := t0; ; i++ {
		l, err := get(3)
		if err != nil {
			return 0, err
		}
		if !(i < l) {
			break
		}
		t1, err := get(0)
		if err != nil {
			return 0, err
		}
		for j := t1; j < 3; j++ {
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
	first := true
	for i := 0; ; {
		if !first {
			n, err := get(1)
			if err != nil {
				return 0, err
			}
			i += n
		}
		first = false
		if !(i < 4) {
			break
		}
		if i == 1 {
			continue
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
	t, err := get(n)
	if err != nil {
		return 0, err
	}
	for i := t; i < 3; i++ {
		continue
	}
	return 1, nil
}

func main() {
	fmt.Println(initCond())
	fmt.Println(postLabel())
	fmt.Println(failIn(1))
	fmt.Println(failIn(99))
}
