// phase: 07

// Error values as type arguments and constraints: T error, custom error types, wrapping, errors.Is and As.
package main

import (
	"errors"
	"fmt"
)

type NotFound struct{ Name string }

func (e NotFound) Error() string { return "not found: " + e.Name }

type Temporary struct{ Op string }

func (e *Temporary) Error() string   { return "temporary: " + e.Op }
func (e *Temporary) Temporary() bool { return true }

var ErrBase = errors.New("base")

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func Try[T any](f func() (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = fmt.Errorf("caught: %w", e)
			} else {
				err = fmt.Errorf("caught: %v", r)
			}
		}
	}()
	return f()
}

func Msgs[E error](es ...E) (out []string) {
	for _, e := range es {
		out = append(out, e.Error())
	}
	return
}

func IsTemp[E error](e E) bool {
	t, ok := any(e).(interface{ Temporary() bool })
	return ok && t.Temporary()
}

func As[T error](err error) (T, bool) {
	var target T
	ok := errors.As(err, &target)
	return target, ok
}

func Wrap[E error](e E, ctx string) error { return fmt.Errorf("%s: %w", ctx, e) }

func main() {
	fmt.Println(Msgs(NotFound{"a"}, NotFound{"b"}), Msgs(&Temporary{"read"}), Msgs[error](ErrBase, NotFound{"c"}), Msgs[error]())
	fmt.Println(IsTemp(&Temporary{}), IsTemp(NotFound{}), IsTemp[error](&Temporary{}), IsTemp[error](ErrBase))
	err := Wrap(NotFound{"file"}, "open")
	err2 := Wrap(Wrap(&Temporary{"net"}, "dial"), "fetch")
	nf, ok1 := As[NotFound](err)
	_, ok2 := As[NotFound](err2)
	tp, ok3 := As[*Temporary](err2)
	fmt.Println(err, nf.Name, ok1, ok2, tp.Op, ok3)
	fmt.Println(errors.Is(Wrap(ErrBase, "x"), ErrBase), errors.Is(err, ErrBase), errors.Is(Wrap[error](ErrBase, "y"), ErrBase))
	fmt.Println(Try(func() (int, error) { return Must(5, nil), nil }))
	fmt.Println(Try(func() (string, error) { return Must("x", NotFound{"z"}), nil }))
	v, err3 := Try(func() ([]int, error) { return Must([]int{1}, &Temporary{"w"}), nil })
	_, ok4 := As[*Temporary](err3)
	fmt.Println(v == nil, err3, ok4)
	fmt.Println(Try(func() (any, error) { panic("plain") }))
}

// Output:
// [not found: a not found: b] [temporary: read] [base not found: c] []
// true false true false
// open: not found: file file true false net true
// true false true
// 5 <nil>
//  caught: not found: z
// true caught: temporary: w true
// <nil> caught: plain
