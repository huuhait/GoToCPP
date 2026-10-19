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

func sum!(xs []string) int {
	if len(xs) == 0 {
		return 0
	}
	return num(xs[0]) + sum(xs[1:])
}

func main() {
	fmt.Println(sum([]string{"1", "2", "3"}))
	fmt.Println(sum([]string{"1", "x", "3"}))
}

// Output:
// 6 <nil>
// 0 num "x": bad
