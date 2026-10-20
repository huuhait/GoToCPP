// phase: 09

package main

import (
	"errors"
	"fmt"
)

var a []any

type S struct{}

func (S) String() string { a[1] = errors.New("x"); a[2] = 0; return "S" }

func main() {
	a = []any{S{}, errors.New("e1"), errors.New("e2")}
	err := fmt.Errorf("%v %w %w", a...)
	fmt.Println(err)
	fmt.Println(errors.Unwrap(err))
	fmt.Println(err.(interface{ Unwrap() []error }).Unwrap())
}

// Output:
// S x %!w(int=0)
// <nil>
// [x]
