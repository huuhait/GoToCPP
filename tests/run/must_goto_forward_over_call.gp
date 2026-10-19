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

func fwd(i string, skip bool) (n int, err error) {
	if skip {
		goto done
	}
	n = num!(i)
	fmt.Println("computed", n)
	if n > 100 {
		return num!(i), nil
	}
done:
	return n, nil
}

func main() {
	fmt.Println(fwd("1", false))
	fmt.Println(fwd("1", true))
	fmt.Println(fwd("x", false))
	fmt.Println(fwd("x", true))
}

// Output:
// computed 1
// 1 <nil>
// 0 <nil>
// 0 num "x": bad
// 0 <nil>
