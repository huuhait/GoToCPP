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

// ok fails for n < 0, with a trace.
func ok(n int) (int, error) {
	fmt.Println("ok", n)
	if n < 0 {
		return 0, fmt.Errorf("ok %d: %w", n, ErrBad)
	}
	return n * 2, nil
}

func f(s string) (string, error) {
	if v := ok!(num!(s)); v > 6 {
		return "a", nil
	} else if w := ok!(v + 1); w > 4 {
		return fmt.Sprint("b", v, w), nil
	}
	return "c", nil
}

func main() {
	fmt.Println(f("4"))
	fmt.Println(f("2"))
	fmt.Println(f("0"))
	fmt.Println(f("x"))
}

// Output:
// ok 4
// a <nil>
// ok 2
// ok 5
// b4 10 <nil>
// ok 0
// ok 1
// c <nil>
//  num "x": bad
