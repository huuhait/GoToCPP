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


func parse![T int | int64](s string) T {
	n := num(s)
	return T(n)
}

func main() {
	fmt.Println(parse[int]("7"))
	fmt.Println(parse![int64]("8"))
	_, err := parse[int]("x")
	fmt.Println(err)
}

// Output:
// 7 <nil>
// 8
// num "x": bad
