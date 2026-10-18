// phase: 05

// errors.ErrUnsupported: its text, identity, and an Is method that makes another error match it.

package main

import (
	"errors"
	"fmt"
)

type noLink struct{}

func (noLink) Error() string        { return "link: not supported here" }
func (noLink) Is(target error) bool { return target == errors.ErrUnsupported }

type wrap struct{ e error }

func (w wrap) Error() string { return "wrap: " + w.e.Error() }
func (w wrap) Unwrap() error { return w.e }

func main() {
	fmt.Println(errors.ErrUnsupported.Error())
	fmt.Println(errors.ErrUnsupported == errors.ErrUnsupported, errors.ErrUnsupported == errors.New("unsupported operation"))
	fmt.Println(errors.Is(noLink{}, errors.ErrUnsupported), errors.Is(wrap{noLink{}}, errors.ErrUnsupported))
	fmt.Println(errors.Is(wrap{errors.ErrUnsupported}, errors.ErrUnsupported), errors.Is(errors.New("x"), errors.ErrUnsupported))
	fmt.Println(errors.Is(errors.ErrUnsupported, noLink{}))
}

// Output:
// unsupported operation
// true false
// true true
// true false
// false
