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

// ok fails for n < 0, with a trace.
func ok(n int) (int, error) {
	fmt.Println("ok", n)
	if n < 0 {
		return 0, fmt.Errorf("ok %d: %w", n, ErrBad)
	}
	return n * 2, nil
}

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

func f(s string) (int, error) {
	a, b := ok!(1), ok!(2)
	var i, v int
	i, v = say("i"), num!(s)
	a, b = b, a
	return a*100 + b*10 + i + v, nil
}

func main() {
	fmt.Println(f("3"))
	fmt.Println(f("x"))
}

// Output:
// ok 1
// ok 2
// say i
// 424 <nil>
// ok 1
// ok 2
// say i
// 0 num "x": bad
