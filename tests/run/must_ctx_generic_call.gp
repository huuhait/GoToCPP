// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

type Num interface{ ~int | ~int64 }

func conv[T Num](s string) (T, error) {
	if len(s) != 1 || s[0] < '0' || s[0] > '9' {
		return 0, fmt.Errorf("conv %q: %w", s, ErrBad)
	}
	return T(s[0] - '0'), nil
}

func both(a, b string) (int, error) {
	x := conv![int](a)
	y := conv![int64](b)
	return x + int(y), nil
}

func main() {
	fmt.Println(both("3", "4"))
	fmt.Println(both("3", "x"))
	fmt.Println(both("x", "4"))
}

// Output:
// 7 <nil>
// 0 conv "x": bad
// 0 conv "x": bad
