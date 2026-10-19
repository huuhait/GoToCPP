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

func two!(a, b string) (x, y int) {
	x = num(a)
	y = num(b)
	return
}

func main() {
	fmt.Println(two("1", "2"))
	fmt.Println(two("1", "z"))
	fmt.Println(two("z", "2"))
}

// Output:
// 1 2 <nil>
// 0 0 num "z": bad
// 0 0 num "z": bad
