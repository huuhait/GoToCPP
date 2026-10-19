package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func trace(s string) string {
	fmt.Println("trace", s)
	return s
}

func done(s string) error {
	fmt.Println("done", s)
	if s == "bad" {
		return fmt.Errorf("done %s: %w", s, ErrBad)
	}
	return nil
}

func f(a, b string) (err error) {
	a1 := trace(a)
	defer func() {
		if e := done(a1); err == nil {
			err = e
		}
	}()
	b1 := trace(b)
	defer func() {
		if e := done(b1); err == nil {
			err = e
		}
	}()
	fmt.Println("body")
	return nil
}

func main() {
	fmt.Println(f("x", "y"))
	fmt.Println(f("x", "bad"))
	fmt.Println(f("bad", "bad"))
}
