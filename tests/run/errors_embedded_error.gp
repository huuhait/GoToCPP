// phase: 05

// A struct that embeds the error interface has Error promoted from it; errors.Is and As see the wrapper, not the embedded one.

package main

import (
	"errors"
	"fmt"
)

var Base = errors.New("base")

type annotated struct {
	error
	where string
}

type annotatedW struct {
	error
	where string
}

func (a annotatedW) Unwrap() error { return a.error }

func main() {
	a := annotated{Base, "here"}
	w := annotatedW{Base, "there"}
	fmt.Println(a.Error(), w.Error(), a.where, w.where)
	fmt.Println(errors.Is(a, Base), errors.Is(w, Base))
	fmt.Println(errors.Unwrap(a) == nil, errors.Unwrap(w) == Base)
	var got annotatedW
	fmt.Println(errors.As(a, &got), errors.As(w, &got), got.where)
	var e error = a
	_, isA := e.(annotated)
	fmt.Println(isA, e == error(a))
}

// Output:
// base base here there
// false true
// true true
// false true there
// true true
