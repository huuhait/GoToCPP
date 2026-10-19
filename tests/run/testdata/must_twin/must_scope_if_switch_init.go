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

func f(a string) (int, error) {
	x := 1
	{
		x, err := num(a)
		if err != nil {
			return 0, err
		}
		if x > 3 {
			fmt.Println("inner", x)
		}
	}
	fmt.Println("outer x", x)
	{
		x, err := num(a)
		if err != nil {
			return 0, err
		}
		switch x {
		case 4:
			fmt.Println("four", x)
		}
	}
	{
		y, err := num(a)
		if err != nil {
			return 0, err
		}
		switch {
		case y > 3:
			fmt.Println("big", y)
		}
	}
	fmt.Println("outer x", x)
	{
		v, err := num(a)
		if err != nil {
			return 0, err
		}
		if v > 100 {
			fmt.Println("never")
		} else {
			w, err := num(a)
			if err != nil {
				return 0, err
			}
			if w > 2 {
				fmt.Println("else if", v, w)
			}
		}
	}
	v := "later"
	y := 2.5
	fmt.Println(v, y)
	return x, nil
}

func main() {
	fmt.Println(f("5"))
	fmt.Println(f("2"))
	fmt.Println(f("x"))
}
