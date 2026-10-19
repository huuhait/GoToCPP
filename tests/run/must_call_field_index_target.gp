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

type Pt struct{ X, Y int }

func (p Pt) String() string { return fmt.Sprint("(", p.X, ",", p.Y, ")") }

type box struct {
	p  Pt
	xs []int
	m  map[string]int
}

func fill(b *box, s string) error {
	b.p.X = num!(s)
	b.xs[1] = num!(s) + 1
	b.m["k"] = num!("9")
	return nil
}

func main() {
	b := &box{xs: make([]int, 3), m: map[string]int{}}
	fmt.Println(fill(b, "4"), b.p, b.xs, b.m)
	fmt.Println(fill(b, "z"), b.p, b.xs, b.m)
}

// Output:
// <nil> (4,0) [0 5 0] map[k:9]
// num "z": bad (4,0) [0 5 0] map[k:9]
