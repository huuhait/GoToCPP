// phase: 05

// Unwrap promoted through embedding a struct, by value and by pointer.

package main

import (
	"errors"
	"fmt"
)

var Root = errors.New("root")

type cause struct{ err error }

func (c cause) Unwrap() error { return c.err }

type pcause struct{ err error }

func (c *pcause) Unwrap() error { return c.err }

type ErrA struct {
	cause
	op string
}

func (e ErrA) Error() string { return e.op + ": " + e.err.Error() }

type ErrB struct {
	*pcause
	op string
}

func (e ErrB) Error() string { return e.op + ": " + e.err.Error() }

type ErrC struct {
	pcause
	op string
}

func (e *ErrC) Error() string { return e.op + "!" }

func main() {
	a := ErrA{cause{Root}, "a"}
	b := ErrB{&pcause{a}, "b"}
	c := &ErrC{pcause{b}, "c"}
	fmt.Println(a.Error(), "|", b.Error(), "|", c.Error())
	fmt.Println(errors.Is(c, Root), errors.Is(c, a), errors.Is(b, Root))
	_, valueHas := any(ErrC{}).(interface{ Unwrap() error })
	fmt.Println(valueHas)
	var ea ErrA
	fmt.Println(errors.As(c, &ea), ea.op)
}

// Output:
// a: root | b: a: root | c!
// true true true
// false
// true a
