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

func read!(name string, n int) (v int, err error) {
	defer errors.Wrapf(&err, "read %s", name)
	r := open(name)
	defer r.Close()
	v = r.Read(n)
	return v, nil
}

func main() {
	fmt.Println(read("a", 2))
	fmt.Println(read("bad", 2))
	fmt.Println(read("a", 9))
	fmt.Println(read("nope", 1))
}

// Output:
// open a
// read a 2
// close a
// 2 <nil>
// open bad
// read bad 2
// close bad
// 2 read bad: close bad: bad
// open a
// read a 9
// close a
// 0 read a: read a: bad
// open nope
// 0 read nope: open nope: bad
