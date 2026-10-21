// phase: 09

// Four forms of a must call (the hand variant): the code that a programmer writes for each form of the
// bang variant.
package main

import (
	"errors"
	"fmt"
)

var errBad = errors.New("bad")

func parse(s string) (int, error) {
	if s == "" {
		return 0, errBad
	}
	return len(s), nil
}

func pair(s string) (int, int, error) {
	n, err := parse(s)
	return n, n * 2, err
}

func check(n int) error {
	if n < 0 {
		return errBad
	}
	return nil
}

func define(s string) (int, error) {
	n, err := parse(s)
	if err != nil {
		return 0, err
	}
	return n + 1, nil
}

func assign(s string) (int, error) {
	total := 0
	{
		v, err := parse(s)
		if err != nil {
			return 0, err
		}
		total = v
	}
	return total, nil
}

func statement(n int) error {
	{
		err := check(n)
		if err != nil {
			return err
		}
	}
	return nil
}

func twoValues(s string) (int, error) {
	a, b, err := pair(s)
	if err != nil {
		return 0, err
	}
	return a + b, nil
}

func main() {
	fmt.Println(define("abc"))
	fmt.Println(assign("abc"))
	fmt.Println(statement(1))
	fmt.Println(twoValues("ab"))
}
