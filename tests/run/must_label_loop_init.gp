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

func f(n string) (int, error) {
	total := 0
outer:
	for i := num!(n); i < 6; i++ {
		for j := num!(n); j < 4; j++ {
			if j == 2 {
				continue outer
			}
			if i == 4 {
				break outer
			}
			total += i*10 + j
		}
	}
	return total, nil
}

func main() {
	fmt.Println(f("1"))
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}

// Output:
// 63 <nil>
// 33 <nil>
// 0 num "x": bad
