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

var ErrBig = errors.New("too big")

func limit!(s string) int {
	n := num(s)
	if n > 5 {
		return 0, fmt.Errorf("limit %d: %w", n, ErrBig)
	}
	return n
}

func main() {
	fmt.Println(limit("3"))
	_, err := limit("9")
	fmt.Println(err, errors.Is(err, ErrBig), errors.Is(err, ErrBad))
	_, err = limit("z")
	fmt.Println(err, errors.Is(err, ErrBig), errors.Is(err, ErrBad))
}

// Output:
// 3 <nil>
// limit 9: too big true false
// num "z": bad false true
