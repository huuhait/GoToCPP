// phase: 09

// The live operands of f(a...) also cover the width and precision taken with * and the operands
// that %w wraps: a String method that stores into the slice changes what they read.
package main

import (
	"errors"
	"fmt"
)

var a []any

type S struct{}

func (S) String() string { a[1] = "x"; a[2] = 0; return "S" }

func main() {
	a = []any{S{}, 6, 1}
	fmt.Printf("%v[%*d]\n", a...)
	a = []any{S{}, 1, errors.New("e")}
	err := fmt.Errorf("%v %v %w", a...)
	fmt.Println(err, errors.Unwrap(err))
}

// Output:
// S[%!(BADWIDTH)0]
// S x %!w(int=0) <nil>
