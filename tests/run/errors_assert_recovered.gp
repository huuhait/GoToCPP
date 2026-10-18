// phase: 05

// The texts of failed assertions on errors of the errors package, as the recovered runtime.Error says them.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct{}

func (*MyErr) Error() string { return "my" }

type Coder interface{ Code() int }

func try(f func()) {
	defer func() {
		r := recover()
		e, ok := r.(error)
		fmt.Println(ok, e.Error())
	}()
	f()
}

func main() {
	a := errors.New("a")
	j := errors.Join(a, errors.New("b"))
	var nilErr error
	try(func() { _ = a.(*MyErr) })
	try(func() { _ = j.(*MyErr) })
	try(func() { _ = nilErr.(*MyErr) })
	try(func() { _ = a.(Coder) })
	try(func() { _ = j.(Coder) })
	try(func() { _ = any(a).(string) })
	try(func() { _ = a.(interface{ Unwrap() error }) })
	_, ok := a.(interface{ Unwrap() []error })
	_, ok2 := j.(interface{ Unwrap() []error })
	fmt.Println(ok, ok2)
}

// Output:
// true interface conversion: error is *errors.errorString, not *main.MyErr
// true interface conversion: error is *errors.joinError, not *main.MyErr
// true interface conversion: error is nil, not *main.MyErr
// true interface conversion: *errors.errorString is not main.Coder: missing method Code
// true interface conversion: *errors.joinError is not main.Coder: missing method Code
// true interface conversion: interface {} is *errors.errorString, not string
// true interface conversion: *errors.errorString is not interface { Unwrap() error }: missing method Unwrap
// false true
