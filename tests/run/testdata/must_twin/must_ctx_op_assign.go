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

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func f(i, j string) (int, error) {
	total := 10
	n, err := num(i)
	if err != nil {
		return 0, err
	}
	total += n
	arr := []int{0, 0, 0, 0}
	idx, err := num(i)
	if err != nil {
		return 0, err
	}
	val, err := num(j)
	if err != nil {
		return 0, err
	}
	arr[idx] += val
	k := say("k")
	v2, err := num(j)
	if err != nil {
		return 0, err
	}
	arr[k] = v2
	return total + arr[1] + arr[3], nil
}

func main() {
	fmt.Println(f("1", "2"))
	fmt.Println(f("1", "x"))
	fmt.Println(f("x", "x"))
}
