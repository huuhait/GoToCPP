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
	defer r.Close!()
	return r.Read!(n), nil
}

func useNamed(name string) (err error) {
	r, err := open(name)
	if err != nil {
		return err
	}
	defer r.Close!()
	return errors.New("body failed")
}

func main() {
	fmt.Println(use("a", 2))
	fmt.Println(use("bad", 2))
	fmt.Println(use("a", 9))
	fmt.Println(use("bad", 9))
	fmt.Println(use("nope", 1))
	fmt.Println(useNamed("bad"))
	fmt.Println(useNamed("a"))
}

// Output:
// open a
// read a 2
// close a
// 2 <nil>
// open bad
// read bad 2
// close bad
// 2 close bad: bad
// open a
// read a 9
// close a
// 0 read a: bad
// open bad
// read bad 9
// close bad
// 0 read bad: bad
// open nope
// 0 open nope: bad
// open bad
// close bad
// body failed
// open a
// close a
// body failed
