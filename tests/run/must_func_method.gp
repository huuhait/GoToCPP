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

type Adder interface {
	Add(s string) error
}

type Sum struct{ total int }

func (s *Sum) Add!(x string) {
	n := num(x)
	s.total += n
}

func (s Sum) Show!() int {
	return s.total
}

func feed(a Adder, xs ...string) error {
	for _, x := range xs {
		if err := a.Add(x); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	s := &Sum{}
	fmt.Println(feed(s, "1", "2", "3"), s.total)
	err := feed(s, "4", "q", "5")
	fmt.Println(err, errors.Is(err, ErrBad), s.total)
	fmt.Println(s.Show())
}

// Output:
// <nil> 6
// num "q": bad true 10
// 10 <nil>
