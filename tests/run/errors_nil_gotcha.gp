// phase: 05

// A nil *MyErr stored in an error is not a nil error; a nil error is.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct{ msg string }

func (e *MyErr) Error() string {
	if e == nil {
		return "nil MyErr"
	}
	return e.msg
}

func bad() error {
	var p *MyErr
	return p
}

func good() error {
	return nil
}

func main() {
	e1, e2 := bad(), good()
	fmt.Println(e1 == nil, e2 == nil)
	fmt.Println(e1.Error())
	fmt.Println(errors.Is(e1, nil), errors.Is(e2, nil), errors.Is(nil, e1))
	var p *MyErr
	fmt.Println(errors.Is(e1, error(p)), e1 == error(p))
	var target *MyErr
	fmt.Println(errors.As(e1, &target), target == nil)
	fmt.Println(errors.Unwrap(e1) == nil)
}

// Output:
// false true
// nil MyErr
// false true false
// true true
// true true
// true
