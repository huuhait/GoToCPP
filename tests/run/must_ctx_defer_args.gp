// phase: 10

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

func f(a, b string) error {
	defer done!(trace(a))
	defer done!(trace(b))
	fmt.Println("body")
	return nil
}

func main() {
	fmt.Println(f("x", "y"))
	fmt.Println(f("x", "bad"))
	fmt.Println(f("bad", "bad"))
}

// Output:
// trace x
// trace y
// body
// done y
// done x
// <nil>
// trace x
// trace bad
// body
// done bad
// done x
// done bad: bad
// trace bad
// trace bad
// body
// done bad
// done bad
// done bad: bad
