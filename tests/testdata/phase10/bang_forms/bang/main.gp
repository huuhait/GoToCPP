// phase: 09

// Four forms of a must call (the bang variant): define, assign, statement, and two values. The hand
// variant is the code that a programmer writes for each of them.
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
	n := parse!(s)
	return n + 1, nil
}

func assign(s string) (int, error) {
	total := 0
	total = parse!(s)
	return total, nil
}

func statement(n int) error {
	check!(n)
	return nil
}

func twoValues(s string) (int, error) {
	a, b := pair!(s)
	return a + b, nil
}

func main() {
	fmt.Println(define("abc"))
	fmt.Println(assign("abc"))
	fmt.Println(statement(1))
	fmt.Println(twoValues("ab"))
}
