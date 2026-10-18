// phase: 05

// The panics of errors.As, recovered: Go's exact texts for every kind of bad target; a nil error is false before any check.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct{ n int }

func (e *MyErr) Error() string { return "myerr" }

type ValOnly struct{}

func (ValOnly) Error() string { return "valonly" }

type NotErr struct{ n int }

type PtrOnly struct{ n int }

func (p *PtrOnly) Error() string { return "ptronly" }

func try(name string, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			fmt.Println(name+":", "no panic")
			return
		}
		s, isString := r.(string)
		fmt.Println(name+":", isString, s)
	}()
	f()
}

func main() {
	err := errors.New("e")
	var i int
	var ne NotErr
	var po PtrOnly
	var s string
	var m MyErr
	var vo ValOnly
	try("nil", func() { errors.As(err, nil) })
	try("int", func() { errors.As(err, 5) })
	try("struct", func() { errors.As(err, MyErr{}) })
	try("nil ptr", func() { errors.As(err, (*MyErr)(nil)) })
	try("nil ptrptr", func() { errors.As(err, (**MyErr)(nil)) })
	try("*int", func() { errors.As(err, &i) })
	try("*string", func() { errors.As(err, &s) })
	try("*NotErr", func() { errors.As(err, &ne) })
	try("*PtrOnly", func() { errors.As(err, &po) })
	try("*MyErr", func() { errors.As(err, &m) })
	try("*[]error", func() { errors.As(err, &[]error{}) })
	try("*func", func() { errors.As(err, new(func())) })
	try("ok value", func() { _ = errors.As(err, &vo) })
	try("ok ptrptr", func() { var p *MyErr; _ = errors.As(err, &p) })
	try("nil err", func() { _ = errors.As(nil, 5) || errors.As(nil, nil) || errors.As(nil, &i) })
}

// Output:
// nil: true errors: target cannot be nil
// int: true errors: target must be a non-nil pointer
// struct: true errors: target must be a non-nil pointer
// nil ptr: true errors: target must be a non-nil pointer
// nil ptrptr: true errors: target must be a non-nil pointer
// *int: true errors: *target must be interface or implement error
// *string: true errors: *target must be interface or implement error
// *NotErr: true errors: *target must be interface or implement error
// *PtrOnly: true errors: *target must be interface or implement error
// *MyErr: true errors: *target must be interface or implement error
// *[]error: true errors: *target must be interface or implement error
// *func: true errors: *target must be interface or implement error
// ok value: no panic
// ok ptrptr: no panic
// nil err: no panic
