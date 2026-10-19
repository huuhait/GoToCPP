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

func f(name string) (n int, err error) {
	ep := &err
	if r, err := open(name); err == nil {
		defer func() {
			if cerr := r.Close(); *ep == nil {
				*ep = cerr
			}
		}()
	}
	return 1, nil
}

func g(name string) (n int, err error) {
	ep := &err
	{
		err := errors.New("shadow")
		_ = err
		r := &res{name}
		defer func() {
			if cerr := r.Close(); *ep == nil {
				*ep = cerr
			}
		}()
	}
	return 2, nil
}

func h(name string) (n int, err error) {
	ep := &err
	for err := 0; err < 1; err++ {
		r := &res{name}
		defer func() {
			if cerr := r.Close(); *ep == nil {
				*ep = cerr
			}
		}()
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
