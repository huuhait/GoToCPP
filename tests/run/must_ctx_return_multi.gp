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

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func two(a, b string) (int, int, error) {
	say("start")
	return num!(a), num!(b), nil
}

func main() {
	fmt.Println(two("1", "2"))
	fmt.Println(two("1", "x"))
	fmt.Println(two("y", "x"))
}

// Output:
// say start
// 1 2 <nil>
// say start
// 0 0 num "x": bad
// say start
// 0 0 num "y": bad
