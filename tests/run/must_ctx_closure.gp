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

func outer(xs []string) (int, error) {
	parse := func(s string) (int, error) {
		n := num!(s)
		return n + 100, nil
	}
	total := 0
	for _, x := range xs {
		v, err := parse(x)
		if err != nil {
			fmt.Println("skip:", err)
			continue
		}
		total += v
	}
	last := num!(xs[len(xs)-1])
	return total + last, nil
}

func main() {
	fmt.Println(outer([]string{"1", "2"}))
	fmt.Println(outer([]string{"1", "x", "3"}))
	fmt.Println(outer([]string{"1", "x"}))
}

// Output:
// 205 <nil>
// skip: num "x": bad
// 207 <nil>
// skip: num "x": bad
// 0 num "x": bad
