// phase: 05

// The errors package: Is, As, Unwrap, Join, custom Is/As methods, multiple %w, panics for bad As targets.
package main

import (
	"errors"
	"fmt"
)

type NotFound struct{ Key string }

func (e NotFound) Error() string { return "not found: " + e.Key }

type Timeout struct{ Op string }

func (e *Timeout) Error() string   { return e.Op + ": timeout" }
func (e *Timeout) Is(t error) bool { _, ok := t.(*Timeout); return ok }
func (e *Timeout) Temporary() bool { return true }

type Multi struct{ errs []error }

func (m Multi) Error() string   { return fmt.Sprint(len(m.errs), " errors") }
func (m Multi) Unwrap() []error { return m.errs }

type As2 struct{ code int }

func (a As2) Error() string { return fmt.Sprint("as2 ", a.code) }
func (a As2) As(target any) bool {
	if p, ok := target.(*NotFound); ok {
		*p = NotFound{fmt.Sprint("via-as-", a.code)}
		return true
	}
	return false
}

var ErrBase = errors.New("base")

func main() {
	e1 := fmt.Errorf("l1: %w", ErrBase)
	e2 := fmt.Errorf("l2: %w", e1)
	e3 := fmt.Errorf("l3: %w + %w", e2, NotFound{"k"})
	fmt.Println(e3)
	fmt.Println(errors.Is(e3, ErrBase), errors.Is(e3, NotFound{"k"}), errors.Is(e3, NotFound{"z"}))
	fmt.Println(errors.Unwrap(e1) == ErrBase, errors.Unwrap(e3) == nil, errors.Unwrap(ErrBase) == nil)
	var nf NotFound
	fmt.Println(errors.As(e3, &nf), nf.Key)
	var tp interface{ Temporary() bool }
	te := fmt.Errorf("ctx: %w", &Timeout{"read"})
	fmt.Println(errors.As(te, &tp), tp != nil && tp.Temporary())
	fmt.Println(errors.Is(te, &Timeout{"other"}), errors.Is(te, ErrBase))
	j := errors.Join(e1, nil, NotFound{"j"}, nil)
	fmt.Printf("%q\n", j.Error())
	fmt.Println(errors.Is(j, ErrBase), errors.Join(nil, nil) == nil, errors.Join() == nil)
	if u, ok := j.(interface{ Unwrap() []error }); ok {
		fmt.Println(len(u.Unwrap()))
	}
	m := Multi{[]error{ErrBase, &Timeout{"w"}}}
	fmt.Println(errors.Is(m, ErrBase), errors.Is(m, &Timeout{}), errors.Is(fmt.Errorf("x: %w", m), ErrBase))
	var nf2 NotFound
	fmt.Println(errors.As(As2{7}, &nf2), nf2)
	var pt *Timeout
	fmt.Println(errors.As(m, &pt), pt)
	wrapped := fmt.Errorf("no wrap: %v", ErrBase)
	fmt.Println(errors.Is(wrapped, ErrBase), errors.Unwrap(wrapped))
	e4 := fmt.Errorf("%w and %w", ErrBase, ErrBase)
	fmt.Println(e4, errors.Is(e4, ErrBase))
	fmt.Printf("%v|%s|%q|%x\n", ErrBase, ErrBase, ErrBase, ErrBase)
	fmt.Printf("%T %T %T\n", ErrBase, e1, e3)
	fmt.Println(errors.New("a") == errors.New("a"), ErrBase == ErrBase)
	for _, tc := range []func(){
		func() { errors.As(ErrBase, nil) },
		func() { var x int; errors.As(ErrBase, &x) },
		func() { errors.As(ErrBase, NotFound{}) },
		func() { var e error; errors.As(ErrBase, e) },
		func() { fmt.Println(errors.Is(ErrBase, nil)) },
	} {
		func() {
			defer func() { fmt.Println("panic:", recover()) }()
			tc()
		}()
	}
	fmt.Println(fmt.Errorf("bad %w", "str"), fmt.Errorf("none %w"), fmt.Errorf("%w", nil))
}
