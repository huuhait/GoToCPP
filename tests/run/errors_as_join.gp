// phase: 05

// errors.As in a Join tree: depth first, children in order, nil entries skipped.

package main

import (
	"errors"
	"fmt"
)

type E struct{ id int }

func (e *E) Error() string { return fmt.Sprint("E", e.id) }
func (e *E) ID() int       { return e.id }

type multi []error

func (m multi) Error() string   { return "multi" }
func (m multi) Unwrap() []error { return m }

type wrap struct{ err error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.err }

func main() {
	e1, e2, e3 := &E{1}, &E{2}, &E{3}
	tree := errors.Join(errors.New("x"), wrap{errors.Join(e1, e2)}, e3)
	var got *E
	fmt.Println(errors.As(tree, &got), got.id)
	tree2 := multi{nil, errors.New("y"), nil, wrap{e3}, e1}
	fmt.Println(errors.As(tree2, &got), got.id)
	var none *wrap
	fmt.Println(errors.As(errors.Join(e1, e2), &none), none == nil)
	var w wrap
	fmt.Println(errors.As(multi{nil, nil}, &w), errors.Is(multi{nil, nil}, e1))
	fmt.Println(errors.As(errors.Join(e1, wrap{e2}), &w), w.err == error(e2))
	var withID interface{ ID() int }
	fmt.Println(errors.As(multi{nil, e2}, &withID), withID.ID(), errors.As(multi{nil, nil, wrap{e3}}, &withID), withID.ID())
}

// Output:
// true 1
// true 3
// false true
// false false
// true true
// true 2 true 3
