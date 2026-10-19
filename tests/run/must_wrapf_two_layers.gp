// phase: 05

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

func f(s string) (n int, err error) {
	defer errors.Wrapf(&err, "first %s", s)
	defer errors.Wrapf(&err, "second")
	n = num!(s)
	return n, nil
}

func main() {
	fmt.Println(f("1"))
	fmt.Println(f("x"))
}

// Output:
// 1 <nil>
// 0 first x: second: num "x": bad
