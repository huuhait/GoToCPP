// phase: 05

// Edge cases of nil: Unwrap returning a nil error ends the chain; a nil *T in the chain; Is with a nil target inside Join.

package main

import (
	"errors"
	"fmt"
)

type link struct{ next error }

func (l *link) Error() string { return "link" }
func (l *link) Unwrap() error { return l.next }

type multi struct{ list []error }

func (m multi) Error() string   { return "multi" }
func (m multi) Unwrap() []error { return m.list }

func main() {
	end := &link{nil}
	fmt.Println(errors.Is(&link{end}, errors.New("x")), errors.Is(end, end), errors.Is(&link{end}, end))
	var nilLink *link
	var asErr error = nilLink
	fmt.Println(asErr != nil, errors.Is(asErr, asErr), errors.Is(&link{asErr}, nilLink))
	m := multi{[]error{nil, end, nil}}
	fmt.Println(errors.Is(m, end), errors.Is(m, nil), errors.Is(multi{}, end))
	fmt.Println(errors.Is(errors.Join(nil, end), nil))
	var got *link
	fmt.Println(errors.As(m, &got), got == end)
	fmt.Println(errors.As(multi{[]error{nil}}, &got), errors.As(multi{nil}, &got))
}

// Output:
// false true true
// true true true
// true false false
// false
// true true
// false false
