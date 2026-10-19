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

func f(a string) (int, error) {
	x := 1
	if x := num!(a); x > 3 {
		fmt.Println("inner", x)
	}
	fmt.Println("outer x", x)
	switch x := num!(a); x {
	case 4:
		fmt.Println("four", x)
	}
	switch y := num!(a); {
	case y > 3:
		fmt.Println("big", y)
	}
	fmt.Println("outer x", x)
	if v := num!(a); v > 100 {
		fmt.Println("never")
	} else if w := num!(a); w > 2 {
		fmt.Println("else if", v, w)
	}
	v := "later"
	y := 2.5
	fmt.Println(v, y)
	return x, nil
}

func main() {
	fmt.Println(f("5"))
	fmt.Println(f("2"))
	fmt.Println(f("x"))
}

// Output:
// inner 5
// outer x 1
// big 5
// outer x 1
// else if 5 5
// later 2.5
// 1 <nil>
// outer x 1
// outer x 1
// later 2.5
// 1 <nil>
// 0 num "x": bad
