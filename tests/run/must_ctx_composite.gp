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

// say prints a trace and returns the length of the name.
func say(name string) int {
	fmt.Println("say", name)
	return len(name)
}

type Pt struct{ X, Y int }

func (p Pt) String() string { return fmt.Sprint("(", p.X, ",", p.Y, ")") }

func build(a, b string) (Pt, []int, map[int]string, error) {
	p := Pt{X: num!(a), Y: num!(b)}
	xs := []int{num!(a), say("mid"), num!(b)}
	m := map[int]string{num!(a): "a", num!(b): "b"}
	return p, xs, m, nil
}

func main() {
	fmt.Println(build("1", "2"))
	fmt.Println(build("1", "x"))
	fmt.Println(build("y", "x"))
}

// Output:
// say mid
// (1,2) [1 3 2] map[1:a 2:b] <nil>
// (0,0) [] map[] num "x": bad
// (0,0) [] map[] num "y": bad
