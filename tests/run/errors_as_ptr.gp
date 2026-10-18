// phase: 05

// errors.As into a pointer-to-struct target: finds the first *MyErr in the chain and stores it.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct {
	Code int
	Msg  string
}

func (e *MyErr) Error() string { return fmt.Sprintf("myerr %d: %s", e.Code, e.Msg) }

type wrap struct{ err error }

func (w wrap) Error() string { return "wrap: " + w.err.Error() }
func (w wrap) Unwrap() error { return w.err }

func main() {
	inner := &MyErr{404, "missing"}
	err := wrap{wrap{inner}}
	var me *MyErr
	fmt.Println(errors.As(err, &me), me == inner, me.Code, me.Msg)
	me.Code = 500
	fmt.Println(inner.Code)
	var other *MyErr
	fmt.Println(errors.As(errors.New("plain"), &other), other == nil)
	first := &MyErr{1, "outer"}
	chain := wrap{first}
	var got *MyErr
	fmt.Println(errors.As(wrap{chain}, &got), got == first)
	fmt.Println(errors.As(first, &got), got == first, errors.As(nil, &got))
}

// Output:
// true true 404 missing
// 500
// false true
// true true
// true true false
