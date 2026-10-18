// phase: 05

// errors.Is: equality, nil handling, a wrapped error that is not the target.

package main

import (
	"errors"
	"fmt"
)

var ErrA = errors.New("a")
var ErrB = errors.New("b")

type wrapper struct {
	msg string
	err error
}

func (w *wrapper) Error() string { return w.msg + ": " + w.err.Error() }
func (w *wrapper) Unwrap() error { return w.err }

func main() {
	fmt.Println(errors.Is(nil, nil))
	fmt.Println(errors.Is(ErrA, nil))
	fmt.Println(errors.Is(nil, ErrA))
	fmt.Println(errors.Is(ErrA, ErrA))
	fmt.Println(errors.Is(ErrA, ErrB))
	w := &wrapper{"outer", ErrA}
	fmt.Println(errors.Is(w, ErrA), errors.Is(w, ErrB), errors.Is(w, w))
	fmt.Println(errors.Is(ErrA, w))
	var e error = w
	fmt.Println(errors.Is(e, e), w.Error())
}

// Output:
// true
// false
// false
// true
// false
// true false true
// false
// true outer: a
