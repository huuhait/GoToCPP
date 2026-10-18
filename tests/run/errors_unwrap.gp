// phase: 05

// errors.Unwrap calls only an Unwrap() error method: not Join's Unwrap() []error, not a method with another type.

package main

import (
	"errors"
	"fmt"
)

type one struct{ e error }

func (o one) Error() string { return "one" }
func (o one) Unwrap() error { return o.e }

type many struct{ es []error }

func (m many) Error() string   { return "many" }
func (m many) Unwrap() []error { return m.es }

type odd struct{}

func (odd) Error() string  { return "odd" }
func (odd) Unwrap() string { return "not an error" }

func main() {
	base := errors.New("base")
	fmt.Println(errors.Unwrap(nil) == nil)
	fmt.Println(errors.Unwrap(base) == nil)
	fmt.Println(errors.Unwrap(one{base}) == base)
	fmt.Println(errors.Unwrap(one{nil}) == nil)
	fmt.Println(errors.Unwrap(many{[]error{base}}) == nil)
	fmt.Println(errors.Unwrap(errors.Join(base, base)) == nil)
	fmt.Println(errors.Unwrap(odd{}) == nil)
	fmt.Println(errors.Unwrap(errors.Unwrap(one{one{base}})) == base)
}

// Output:
// true
// true
// true
// true
// true
// true
// true
// true
