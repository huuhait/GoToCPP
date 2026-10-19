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


func calc!(a, b string) int {
	return num(a)*10 + num(b)
}

func main() {
	fmt.Println(calc("4", "2"))
	_, err := calc("4", "x")
	fmt.Println(err)
	_, err = calc("y", "x")
	fmt.Println(err)
}

// Output:
// 42 <nil>
// num "x": bad
// num "y": bad
