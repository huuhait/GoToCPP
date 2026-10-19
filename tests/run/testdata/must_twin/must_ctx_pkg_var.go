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

// pair splits "a,b" into two digits.
func pair(s string) (int, int, error) {
	if len(s) == 3 && s[1] == ',' {
		a, err := num(s[:1])
		if err != nil {
			return 0, 0, err
		}
		b, err := num(s[2:])
		return a, b, err
	}
	return 0, 0, fmt.Errorf("pair %q: %w", s, ErrBad)
}

func must1(n int, err error) int {
	if err != nil {
		panic(err)
	}
	return n
}

func must2(a, b int, err error) (int, int) {
	if err != nil {
		panic(err)
	}
	return a, b
}

var seven = must1(num("7"))

var a, b = must2(pair("2,5"))

var table = []int{must1(num("1")), must1(num("2")), seven}

func main() {
	fmt.Println(seven, a, b, table)
}
