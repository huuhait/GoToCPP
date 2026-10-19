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

func f(a, b string) (n int, err error) {
	defer func() { fmt.Println("defer 1: n =", n, "err =", err) }()
	defer fmt.Println("defer 2")
	n = num!(a)
	fmt.Println("after a")
	n += num!(b)
	return n, nil
}

func main() {
	fmt.Println(f("1", "2"))
	fmt.Println(f("1", "x"))
	fmt.Println(f("x", "1"))
}

// Output:
// after a
// defer 2
// defer 1: n = 3 err = <nil>
// 3 <nil>
// after a
// defer 2
// defer 1: n = 0 err = num "x": bad
// 0 num "x": bad
// defer 2
// defer 1: n = 0 err = num "x": bad
// 0 num "x": bad
