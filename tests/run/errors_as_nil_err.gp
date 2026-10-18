// phase: 05

// errors.As of a nil error is false whatever the target is: it returns before it looks at it.

package main

import (
	"errors"
	"fmt"
)

type E struct{}

func (*E) Error() string { return "E" }

func main() {
	var e *E
	var s string
	fmt.Println(errors.As(nil, nil), errors.As(nil, 1), errors.As(nil, &s), errors.As(nil, &e), e == nil)
	var err error
	fmt.Println(errors.As(err, &e), errors.As(err, "x"))
	fmt.Println(errors.As(error((*E)(nil)), &e), e == nil)
}

// Output:
// false false false false true
// false false
// true true
