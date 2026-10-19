// phase: 10

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

func f(s string) (int, error) {
	n, err := num(s)
	fmt.Println("first:", n, err)
	m := num!("2")
	k, err := num(s + s)
	fmt.Println("second:", k, err)
	if err != nil {
		err := fmt.Errorf("inner: %w", err)
		fmt.Println(err)
	}
	j := num!(s)
	return n + m + k + j, err
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}

// Output:
// first: 3 <nil>
// second: 0 num "33": bad
// inner: num "33": bad
// 8 num "33": bad
// first: 0 num "x": bad
// second: 0 num "xx": bad
// inner: num "xx": bad
// 0 num "x": bad
