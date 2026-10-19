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

func f(a int, b string, c bool) (err error) {
	defer Wrapf(&err, "a=%d b=%s c=%t pct=%d%%", a, b, c, 50)
	return errors.New("boom")
}

func main() {
	fmt.Println(f(1, "two", true))
}
