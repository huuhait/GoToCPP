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

// ok fails for n < 0, with a trace.
func ok(n int) (int, error) {
	fmt.Println("ok", n)
	if n < 0 {
		return 0, fmt.Errorf("ok %d: %w", n, ErrBad)
	}
	return n * 2, nil
}

func classify(s string, t string) (string, error) {
	{
		n, err := num(s)
		if err != nil {
			return "", err
		}
		if n > 8 {
			return "huge", nil
		}
	}
	{
		n, err := num(s)
		if err != nil {
			return "", err
		}
		k, err := ok(n)
		if err != nil {
			return "", err
		}
		if k > 6 {
			return "big", nil
		}
	}
	{
		n, err := num(t)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "zero", nil
		}
	}
	return "small", nil
}

func tag(s string) (string, error) {
	v, err := num(s)
	if err != nil {
		return "", err
	}
	if v == 1 {
		return "one", nil
	}
	k, err := ok(2)
	if err != nil {
		return "", err
	}
	if v == k {
		return "four?", nil
	}
	return "other", nil
}

func main() {
	fmt.Println(classify("9", "x"))
	fmt.Println(classify("4", "x"))
	fmt.Println(classify("1", "0"))
	fmt.Println(classify("1", "x"))
	fmt.Println(classify("1", "5"))
	fmt.Println(tag("1"))
	fmt.Println(tag("4"))
	fmt.Println(tag("7"))
	fmt.Println(tag("z"))
}
