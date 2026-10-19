// phase: 05

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

func use(name string, n int) (int, error) {
	r := open!(name)
	defer fmt.Println("use done", name)
	got := r.Read!(n)
	r.Close!()
	return got, nil
}

func main() {
	fmt.Println(use("a", 2))
	fmt.Println(use("nope", 2))
	fmt.Println(use("b", 9))
	fmt.Println(use("bad", 1))
}

// Output:
// open a
// read a 2
// close a
// use done a
// 2 <nil>
// open nope
// 0 open nope: bad
// open b
// read b 9
// use done b
// 0 read b: bad
// open bad
// read bad 1
// close bad
// use done bad
// 0 close bad: bad
