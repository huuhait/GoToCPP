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
	x, err := num(a)
	if err != nil {
		return Pt{}, nil, nil, err
	}
	y, err := num(b)
	if err != nil {
		return Pt{}, nil, nil, err
	}
	p := Pt{X: x, Y: y}
	n1, err := num(a)
	if err != nil {
		return Pt{}, nil, nil, err
	}
	mid := say("mid")
	n2, err := num(b)
	if err != nil {
		return Pt{}, nil, nil, err
	}
	xs := []int{n1, mid, n2}
	k1, err := num(a)
	if err != nil {
		return Pt{}, nil, nil, err
	}
	k2, err := num(b)
	if err != nil {
		return Pt{}, nil, nil, err
	}
	m := map[int]string{k1: "a", k2: "b"}
	return p, xs, m, nil
}

func main() {
	fmt.Println(build("1", "2"))
	fmt.Println(build("1", "x"))
	fmt.Println(build("y", "x"))
}
