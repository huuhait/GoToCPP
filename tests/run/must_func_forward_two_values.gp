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

func pairs(s string) (int, string, error) {
	n, err := num(s)
	return n, s + s, err
}

func fw!(s string) (int, string) {
	return pairs(s)
}

func main() {
	fmt.Println(fw("4"))
	fmt.Println(fw("x"))
}

// Output:
// 4 44 <nil>
// 0 xx num "x": bad
