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

func grid(rows []string) (int, error) {
	total := 0
outer:
	for i := 0; i < len(rows); i++ {
		for j := 0; j < len(rows[i]); j = j + 1 {
			v := num!(rows[i][j : j+1])
			if v == 0 {
				continue outer
			}
			if v > 7 {
				break outer
			}
			total += ok!(v)
		}
	}
	return total, nil
}

func main() {
	fmt.Println(grid([]string{"12", "30", "45"}))
	fmt.Println(grid([]string{"12", "8", "45"}))
	fmt.Println(grid([]string{"12", "1x", "45"}))
}

// Output:
// ok 1
// ok 2
// ok 3
// ok 4
// ok 5
// 30 <nil>
// ok 1
// ok 2
// 6 <nil>
// ok 1
// ok 2
// ok 1
// 0 num "x": bad
