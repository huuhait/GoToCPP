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

func pick(a, b string) (string, error) {
	if num!(a) > 5 {
		return "big", nil
	} else if ok!(num!(b)) > 6 {
		return "second", nil
	} else if ok!(-num!(b)) > 0 {
		return "never", nil
	}
	return "none", nil
}

func main() {
	fmt.Println(pick("7", "x"))
	fmt.Println(pick("1", "4"))
	fmt.Println(pick("1", "1"))
	fmt.Println(pick("1", "x"))
	fmt.Println(pick("q", "1"))
}

// Output:
// big <nil>
// ok 4
// second <nil>
// ok 1
// ok -1
//  ok -1: bad
//  num "x": bad
//  num "q": bad
