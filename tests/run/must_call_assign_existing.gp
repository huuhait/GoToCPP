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

// pair splits "a,b" into two digits.
func pair(s string) (int, int, error) {
	if len(s) == 3 && s[1] == ',' {
		a, err := num(s[:1])
		if err != nil {
			return 0, 0, err
		}
		b, err := num(s[2:])
		return a, b, err
	}
	return 0, 0, fmt.Errorf("pair %q: %w", s, ErrBad)
}

func f(s string) (x, y int, err error) {
	x = num!(s)
	x, y = pair!(s + ",5")
	y += num!("1")
	return x, y, nil
}

func main() {
	fmt.Println(f("2"))
	fmt.Println(f("k"))
}

// Output:
// 2 6 <nil>
// 0 0 num "k": bad
