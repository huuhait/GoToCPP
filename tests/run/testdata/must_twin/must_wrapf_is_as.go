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

type CodeErr struct{ Code int }

func (e *CodeErr) Error() string { return fmt.Sprint("code ", e.Code) }

var ErrBase = errors.New("base")

func fail(kind int) error {
	switch kind {
	case 1:
		return ErrBase
	case 2:
		return &CodeErr{42}
	}
	return nil
}

func run(kind int) (err error) {
	defer Wrapf(&err, "run %d", kind)
	if err := fail(kind); err != nil {
		return err
	}
	return nil
}

func main() {
	for kind := 0; kind < 3; kind++ {
		err := run(kind)
		var ce *CodeErr
		fmt.Println(kind, err, errors.Is(err, ErrBase), errors.As(err, &ce))
		if ce != nil {
			fmt.Println("code", ce.Code)
		}
	}
}
