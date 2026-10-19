// phase: 10

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// res is something that closes with an error for the name "bad".
type res struct{ name string }

func open(name string) (*res, error) {
	fmt.Println("open", name)
	if name == "nope" {
		return nil, fmt.Errorf("open %s: %w", name, ErrBad)
	}
	return &res{name}, nil
}

func (r *res) Close() error {
	fmt.Println("close", r.name)
	if r.name == "bad" {
		return fmt.Errorf("close %s: %w", r.name, ErrBad)
	}
	return nil
}

func (r *res) Read(n int) (int, error) {
	fmt.Println("read", r.name, n)
	if n > 3 {
		return 0, fmt.Errorf("read %s: %w", r.name, ErrBad)
	}
	return n, nil
}

type Reader interface {
	Read(n int) (int, error)
	Close() error
}

func drain(rd Reader, ns ...int) (int, error) {
	defer rd.Close!()
	sum := 0
	for _, n := range ns {
		sum += rd.Read!(n)
	}
	return sum, nil
}

func main() {
	fmt.Println(drain(&res{"a"}, 1, 2, 3))
	fmt.Println(drain(&res{"bad"}, 1, 2))
	fmt.Println(drain(&res{"a"}, 1, 9, 3))
}

// Output:
// read a 1
// read a 2
// read a 3
// close a
// 6 <nil>
// read bad 1
// read bad 2
// close bad
// 3 close bad: bad
// read a 1
// read a 9
// close a
// 0 read a: bad
