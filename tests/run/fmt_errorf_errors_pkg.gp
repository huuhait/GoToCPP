// phase: 05

// Errors made by Errorf work with package errors: Is and As find the wrapped errors through
// Unwrap() error and Unwrap() []error, errors.Unwrap peels one %w, Join and Errorf mix; %T names
// stay Go's.
package main

import (
	"errors"
	"fmt"
)

type NotFound struct{ Name string }

func (e *NotFound) Error() string { return "not found: " + e.Name }

type Temp struct{}

func (Temp) Error() string   { return "temp" }
func (Temp) Temporary() bool { return true }

var ErrBase = errors.New("base")

func main() {
	e1 := fmt.Errorf("layer1: %w", ErrBase)
	e2 := fmt.Errorf("layer2: %w", e1)
	fmt.Println(e2, errors.Is(e2, ErrBase), errors.Is(e2, e1), errors.Is(e1, e2))
	fmt.Println(errors.Unwrap(e2) == e1, errors.Unwrap(e1) == ErrBase, errors.Unwrap(ErrBase) == nil)
	nf := &NotFound{"x"}
	e3 := fmt.Errorf("lookup: %w", nf)
	var target *NotFound
	fmt.Println(errors.As(e3, &target), target == nf, target.Name)
	var tmp interface{ Temporary() bool }
	e4 := fmt.Errorf("a: %w, b: %w", Temp{}, ErrBase)
	fmt.Println(e4, errors.Is(e4, ErrBase), errors.As(e4, &tmp), tmp != nil && tmp.Temporary())
	fmt.Println(errors.Unwrap(e4) == nil)
	e5 := errors.Join(e1, fmt.Errorf("other"), nil)
	fmt.Printf("%q %v\n", e5.Error(), errors.Is(e5, ErrBase))
	e6 := fmt.Errorf("outer: %w", e5)
	fmt.Println(errors.Is(e6, ErrBase), fmt.Sprint(e6) == "outer: layer1: base\nother")
	fmt.Printf("%T %T %T %T %T\n", e1, e4, ErrBase, e5, fmt.Errorf("plain"))
	fmt.Println(errors.Is(fmt.Errorf("%w", nil), nil), fmt.Errorf("no wrap %v", ErrBase) == ErrBase, errors.Is(fmt.Errorf("no wrap %v", ErrBase), ErrBase))
}

// Output:
// layer2: layer1: base true true false
// true true true
// true true x
// a: temp, b: base true true true
// true
// "layer1: base\nother" true
// true true
// *fmt.wrapError *fmt.wrapErrors *errors.errorString *errors.joinError *errors.errorString
// false false false
