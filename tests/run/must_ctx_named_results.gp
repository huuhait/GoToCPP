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

func f(a, b string) (x, y int, err error) {
	x = num!(a)
	y = num!(b)
	return
}

func main() {
	fmt.Println(f("1", "2"))
	fmt.Println(f("1", "z"))
	fmt.Println(f("z", "2"))
}

// Output:
// 1 2 <nil>
// 0 0 num "z": bad
// 0 0 num "z": bad
