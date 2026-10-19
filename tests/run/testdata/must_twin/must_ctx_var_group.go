package main

import (
	"errors"
	"fmt"
)

var trace []string

func step(name string, n int, fail bool) (int, error) {
	trace = append(trace, name)
	if fail {
		return 0, errors.New("fail " + name)
	}
	return n, nil
}

func two(name string, a, b int, fail bool) (int, int, error) {
	trace = append(trace, name)
	if fail {
		return 0, 0, errors.New("fail " + name)
	}
	return a, b, nil
}

func f(failAt string) (string, error) {
	a, e1 := step("a", 1, failAt == "a")
	if e1 != nil {
		return "", e1
	}
	plain := len(trace)
	b, c, e2 := two("b", a+10, plain, failAt == "b")
	if e2 != nil {
		return "", e2
	}
	var d int
	d, e3 := step("d", b+c, failAt == "d")
	if e3 != nil {
		return "", e3
	}
	return fmt.Sprint(a, plain, b, c, d, trace), nil
}

func g(fail bool) (int, error) {
	if true {
		x := 5
		y, z, e := two("g", x, x*2, fail)
		if e != nil {
			return 0, e
		}
		return x + y + z, nil
	}
	return 0, nil
}

func main() {
	for _, at := range []string{"", "a", "b", "d"} {
		trace = nil
		s, err := f(at)
		fmt.Printf("%q %v %v\n", s, err, trace)
	}
	fmt.Println(g(false))
	fmt.Println(g(true))
}
