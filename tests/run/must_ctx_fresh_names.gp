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
	must_t1 := 100
	must_e1 := "e"
	must_err := 7
	n := num!(s)
	return n + must_t1 + must_err + len(must_e1), nil
}

func main() {
	fmt.Println(f("1"))
	fmt.Println(f("z"))
}

// Output:
// 109 <nil>
// 0 num "z": bad
