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

func drain(rd Reader, ns ...int) (sum int, err error) {
	defer func() {
		if cerr := rd.Close(); err == nil {
			err = cerr
		}
	}()
	for _, n := range ns {
		v, err := rd.Read(n)
		if err != nil {
			return 0, err
		}
		sum += v
	}
	return sum, nil
}

func main() {
	fmt.Println(drain(&res{"a"}, 1, 2, 3))
	fmt.Println(drain(&res{"bad"}, 1, 2))
	fmt.Println(drain(&res{"a"}, 1, 9, 3))
}
