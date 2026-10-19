// phase: 10

package main

import (
	"errors"
	"fmt"
)

var state string

var ErrBad = errors.New("bad")

// num parses one digit.
func num(s string) (int, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), nil
	}
	return 0, fmt.Errorf("num %q: %w", s, ErrBad)
}

func init!() {
	n := num("3")
	state = fmt.Sprint("init ", n)
}

func main() {
	fmt.Println(state)
}

// Output:
// init 3
