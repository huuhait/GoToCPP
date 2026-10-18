// phase: 05

// An As(any) bool method converts an error to the target's type; it is called with the target pointer.

package main

import (
	"errors"
	"fmt"
)

type Public struct{ text string }

func (p *Public) Error() string { return "public: " + p.text }

type internal struct{ detail string }

func (e internal) Error() string { return "internal: " + e.detail }
func (e internal) As(target any) bool {
	switch t := target.(type) {
	case **Public:
		*t = &Public{"sanitized"}
		return true
	}
	return false
}

type wrap struct{ err error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.err }

func main() {
	err := wrap{internal{"secret"}}
	var p *Public
	fmt.Println(errors.As(err, &p), p.text)
	var other interface{ Foo() }
	fmt.Println(errors.As(err, &other), other == nil)
	fmt.Println(errors.As(wrap{errors.New("no")}, &p), p.text)
	var pp *Public
	fmt.Println(errors.As(wrap{wrap{internal{"deep"}}}, &pp), pp.text)
}

// Output:
// true sanitized
// false true
// false sanitized
// true sanitized
