package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// chk prints its name and fails for "bad".
func chk(name string, v bool) (bool, error) {
	fmt.Println("chk", name, v)
	if name == "bad" {
		return false, fmt.Errorf("chk %s: %w", name, ErrBad)
	}
	return v, nil
}

func and(a, b string, x, y bool) (bool, error) {
	l, err := chk(a, x)
	if err != nil {
		return false, err
	}
	cond := l
	if cond {
		r, err := chk(b, y)
		if err != nil {
			return false, err
		}
		cond = r
	}
	if cond {
		return true, nil
	}
	return false, nil
}

func or(a, b string, x, y bool) (bool, error) {
	l, err := chk(a, x)
	if err != nil {
		return false, err
	}
	res := l
	if !res {
		r, err := chk(b, y)
		if err != nil {
			return false, err
		}
		res = r
	}
	return res, nil
}

func main() {
	fmt.Println(and("a", "b", true, true))
	fmt.Println(and("a", "bad", false, true))
	fmt.Println(and("a", "bad", true, true))
	fmt.Println(or("a", "bad", true, false))
	fmt.Println(or("a", "b", false, true))
	fmt.Println(or("bad", "b", false, true))
	fmt.Println(or("a", "bad", false, true))
}
