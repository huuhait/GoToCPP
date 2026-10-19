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

func f!(name string) (n int, err error) {
	if r, err := open(name); err == nil {
		defer r.Close()
	}
	return 1, nil
}

func g(name string) (n int, err error) {
	{
		err := errors.New("shadow")
		_ = err
		r := &res{name}
		defer r.Close!()
	}
	return 2, nil
}

func h!(name string) (n int, err error) {
	for err := 0; err < 1; err++ {
		r := &res{name}
		defer r.Close()
	}
	return 3, nil
}

func main() {
	fmt.Println(f("a"))
	fmt.Println(f("bad"))
	fmt.Println(g("a"))
	fmt.Println(g("bad"))
	fmt.Println(h("a"))
	fmt.Println(h("bad"))
}

// Output:
// open a
// close a
// 1 <nil>
// open bad
// close bad
// 1 close bad: bad
// close a
// 2 <nil>
// close bad
// 2 close bad: bad
// close a
// 3 <nil>
// close bad
// 3 close bad: bad
