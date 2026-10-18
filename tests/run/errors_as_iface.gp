// phase: 05

// errors.As into an interface target: the first error in the tree that has the method.

package main

import (
	"errors"
	"fmt"
)

type Temporary interface{ Temporary() bool }

type Coder interface {
	error
	Code() int
}

type netErr struct{ temp bool }

func (e netErr) Error() string   { return "net" }
func (e netErr) Temporary() bool { return e.temp }

type httpErr struct{ code int }

func (e *httpErr) Error() string { return "http" }
func (e *httpErr) Code() int     { return e.code }

type wrap struct{ err error }

func (w wrap) Error() string { return "wrap" }
func (w wrap) Unwrap() error { return w.err }

func main() {
	var t Temporary
	err := wrap{wrap{netErr{true}}}
	fmt.Println(errors.As(err, &t), t.Temporary())
	t = nil
	fmt.Println(errors.As(wrap{errors.New("x")}, &t), t == nil)
	var c Coder
	fmt.Println(errors.As(wrap{&httpErr{418}}, &c), c.Code(), c.Error())
	_, isNet := any(c).(netErr)
	_, isHTTP := any(c).(*httpErr)
	fmt.Println(isNet, isHTTP)
	var c2 Coder
	fmt.Println(errors.As(wrap{netErr{}}, &c2), c2 == nil)
}

// Output:
// true true
// false true
// true 418 http
// false true
// false true
