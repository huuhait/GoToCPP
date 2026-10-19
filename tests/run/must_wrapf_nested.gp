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

func inner!(s string) (n int, err error) {
	defer errors.Wrapf(&err, "inner(%s)", s)
	n = num(s)
	return n, nil
}

func middle!(s string) (n int, err error) {
	defer errors.Wrapf(&err, "middle")
	n = inner(s) + 1
	return n, nil
}

func outer(s string) (n int, err error) {
	defer errors.Wrapf(&err, "outer")
	n = middle!(s) + 1
	return n, nil
}

func main() {
	fmt.Println(outer("1"))
	n, err := outer("q")
	fmt.Println(n, err, errors.Is(err, ErrBad))
}

// Output:
// 3 <nil>
// 0 outer: middle: inner(q): num "q": bad true
