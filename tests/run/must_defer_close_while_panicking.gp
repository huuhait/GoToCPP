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

func body(name string, boom bool) {
	r := open!(name)
	defer r.Close!()
	if boom {
		panic("body panic")
	}
	fmt.Println("body done")
}

func try(name string, boom bool) {
	defer func() {
		fmt.Println("recovered:", recover())
	}()
	body(name, boom)
}

func main() {
	try("a", false)
	try("bad", false)
	try("a", true)
	try("bad", true)
	try("nope", true)
}

// Output:
// open a
// body done
// close a
// recovered: <nil>
// open bad
// body done
// close bad
// recovered: close bad: bad
// open a
// close a
// recovered: body panic
// open bad
// close bad
// recovered: body panic
// open nope
// recovered: open nope: bad
