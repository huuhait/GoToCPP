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


func parse[T int | int64](s string) (T, error) {
	n, err := num(s)
	if err != nil {
		return 0, err
	}
	return T(n), nil
}

func main() {
	fmt.Println(parse[int]("7"))
	v, err := parse[int64]("8")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(v)
	_, err = parse[int]("x")
	fmt.Println(err)
}
