// phase: 10
// ExitCode: 2
// Stderr: (?m)^panic: .*num "x": bad

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

func mustNum(s string) int {
	return num!(s) * 2
}

func main() {
	fmt.Println(mustNum("4"))
	fmt.Println(mustNum("x"))
	fmt.Println("not reached")
}

// Output:
// 8
