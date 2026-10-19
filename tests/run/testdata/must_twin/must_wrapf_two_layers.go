package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

// num parses one digit.
func num(s string) (int, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return int(s[0] - '0'), nil
	}
	return 0, fmt.Errorf("num %q: %w", s, ErrBad)
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

func f(s string) (n int, err error) {
	defer Wrapf(&err, "first %s", s)
	defer Wrapf(&err, "second")
	n, err = num(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func main() {
	fmt.Println(f("1"))
	fmt.Println(f("x"))
}
