// phase: 05

// recover() returns the very error that was passed to panic: same pointer, usable with errors.Is and errors.As.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct{ code int }

func (e *MyErr) Error() string { return fmt.Sprint("myerr ", e.code) }

var Sentinel = errors.New("sentinel")

func run(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(error)
			if !ok {
				err = errors.New(fmt.Sprint("not an error: ", r))
				return
			}
			err = e
		}
	}()
	f()
	return nil
}

func main() {
	e1 := &MyErr{7}
	got := run(func() { panic(e1) })
	fmt.Println(got == error(e1), got.Error())
	got = run(func() { panic(fmt.Sprint("a ", "string")) })
	fmt.Println(got.Error())
	got = run(func() { panic(Sentinel) })
	fmt.Println(got == Sentinel, errors.Is(got, Sentinel))
	got = run(func() { panic(fmt.Sprint("wrapped: ", 1)) })
	var me *MyErr
	fmt.Println(errors.As(got, &me), me == nil)
	got = run(func() { var m map[string]int; m["x"] = 1 })
	fmt.Println(got.Error())
	fmt.Println(run(func() {}) == nil)
	got = run(func() { panic(errors.Join(e1, Sentinel)) })
	fmt.Println(errors.As(got, &me), me == e1, errors.Is(got, Sentinel))
}

// Output:
// true myerr 7
// not an error: a string
// true true
// false true
// assignment to entry in nil map
// true
// true true true
