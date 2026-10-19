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
	r, err := open(name)
	if err != nil {
		return 0, err
	}
	defer fmt.Println("use done", name)
	got, err := r.Read(n)
	if err != nil {
		return 0, err
	}
	if err := r.Close(); err != nil {
		return 0, err
	}
	return got, nil
}

func main() {
	fmt.Println(use("a", 2))
	fmt.Println(use("nope", 2))
	fmt.Println(use("b", 9))
	fmt.Println(use("bad", 1))
}
