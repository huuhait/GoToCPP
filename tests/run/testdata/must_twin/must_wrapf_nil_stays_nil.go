package main

import (
	"errors"
	"fmt"
)

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

func f(fail bool) (err error) {
	defer Wrapf(&err, "context")
	if fail {
		return errors.New("inner")
	}
	return nil
}

func main() {
	err := f(false)
	fmt.Println(err == nil, err)
	err = f(true)
	fmt.Println(err == nil, err)
}
