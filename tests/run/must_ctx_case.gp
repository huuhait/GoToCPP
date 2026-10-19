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

func classify(s string, t string) (string, error) {
	switch {
	case num!(s) > 8:
		return "huge", nil
	case ok!(num!(s)) > 6:
		return "big", nil
	case num!(t) == 0:
		return "zero", nil
	}
	return "small", nil
}

func tag(s string) (string, error) {
	switch num!(s) {
	case 1:
		return "one", nil
	case ok!(2):
		return "four?", nil
	default:
		return "other", nil
	}
}

func main() {
	fmt.Println(classify("9", "x"))
	fmt.Println(classify("4", "x"))
	fmt.Println(classify("1", "0"))
	fmt.Println(classify("1", "x"))
	fmt.Println(classify("1", "5"))
	fmt.Println(tag("1"))
	fmt.Println(tag("4"))
	fmt.Println(tag("7"))
	fmt.Println(tag("z"))
}

// Output:
// huge <nil>
// ok 4
// big <nil>
// ok 1
// zero <nil>
// ok 1
//  num "x": bad
// ok 1
// small <nil>
// one <nil>
// ok 2
// four? <nil>
// ok 2
// other <nil>
//  num "z": bad
