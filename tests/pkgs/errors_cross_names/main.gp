// phase: 06

// Types with the same name in different packages (even with the same package name) are different error types: type switches, assertion panics, errors.As.

package main

import (
	"errors"
	"fmt"

	xa "example.com/names/x/a"
	"example.com/names/b"
	ya "example.com/names/y/a"
)

type Err struct{ N int }

func (e *Err) Error() string { return "main.Err" }

func which(err error) string {
	switch e := err.(type) {
	case *xa.Err:
		return fmt.Sprint("x/a ", e.N)
	case *ya.Err:
		return fmt.Sprint("y/a ", e.N)
	case *b.Err:
		return fmt.Sprint("b ", e.N)
	case *Err:
		return fmt.Sprint("main ", e.N)
	}
	return "unknown " + err.Error()
}

func try(f func()) {
	defer func() {
		r := recover()
		fmt.Println(r.(error).Error())
	}()
	f()
}

func main() {
	errs := []error{xa.Make(), ya.Make(), b.Make(), &Err{4}, b.Plain(), xa.Own()}
	for _, e := range errs {
		fmt.Println(which(e))
	}
	var px *xa.Err
	var py *ya.Err
	for _, e := range errs[:4] {
		fmt.Println(errors.As(e, &px), errors.As(e, &py))
	}
	fmt.Println(px.N, py.N)
	fmt.Println(errors.Is(xa.Sentinel, ya.Sentinel), xa.Sentinel == ya.Sentinel)
	try(func() { _ = errs[0].(*ya.Err) })
	try(func() { _ = errs[1].(*b.Err) })
	try(func() { _ = errs[3].(*xa.Err) })
	try(func() { _ = errs[5].(*xa.Err) })
	try(func() { _ = errs[4].(interface{ Code() int }) })
}

// Output:
// x/a 1
// y/a 2
// b 3
// main 4
// unknown b plain
// unknown mine
// true false
// false true
// false false
// false false
// 1 2
// false false
// interface conversion: error is *a.Err, not *a.Err (types from different packages)
// interface conversion: error is *a.Err, not *b.Err
// interface conversion: error is *main.Err, not *a.Err
// interface conversion: error is *a.errorString, not *a.Err
// interface conversion: *errors.errorString is not interface { Code() int }: missing method Code
