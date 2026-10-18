// phase: 05

// errors.As with a target of type error or any: the outermost error matches.

package main

import (
	"errors"
	"fmt"
)

type wrap struct{ err error }

func (w wrap) Error() string { return "wrap(" + w.err.Error() + ")" }
func (w wrap) Unwrap() error { return w.err }

func main() {
	base := errors.New("base")
	err := wrap{wrap{base}}
	var e error
	fmt.Println(errors.As(err, &e), e == err, e.Error())
	var a any
	fmt.Println(errors.As(err, &a), a == any(err))
	_, ok := a.(wrap)
	fmt.Println(ok)
	var s interface{ String() string }
	fmt.Println(errors.As(err, &s), s == nil)
	var e2 error
	fmt.Println(errors.As(base, &e2), e2 == base)
}

// Output:
// true true wrap(wrap(base))
// true true
// true
// false true
// true true
