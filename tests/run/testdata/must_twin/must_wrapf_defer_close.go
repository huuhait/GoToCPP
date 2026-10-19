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

// Wrapf is errors.Wrapf of gocpp, in plain Go.
type wrapfError struct {
	msg string
	err error
}

func (e *wrapfError) Error() string { return e.msg + ": " + e.err.Error() }
func (e *wrapfError) Unwrap() error { return e.err }

func Wrapf(errp *error, format string, args ...any) {
	if *errp == nil {
		return
	}
	*errp = &wrapfError{msg: fmt.Sprintf(format, args...), err: *errp}
}

func read(name string, n int) (v int, err error) {
	defer Wrapf(&err, "read %s", name)
	r, err := open(name)
	if err != nil {
		return 0, err
	}
	defer func() {
		if cerr := r.Close(); err == nil {
			err = cerr
		}
	}()
	v, err = r.Read(n)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func main() {
	fmt.Println(read("a", 2))
	fmt.Println(read("bad", 2))
	fmt.Println(read("a", 9))
	fmt.Println(read("nope", 1))
}
