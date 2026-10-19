package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func main() {
	var err error
	func() {
		r, e := open("bad")
		if e != nil {
			err = e
			return
		}
		defer func() {
			if cerr := r.Close(); err == nil {
				err = cerr
			}
		}()
		v, e := r.Read(2)
		if e != nil {
			err = e
			return
		}
		fmt.Println("read", v)
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}
