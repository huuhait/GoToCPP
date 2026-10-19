// phase: 08

// The run-time panics are runtime.Error values: r.(runtime.Error) works, the text is Go's, and
// an error from errors.New is not one.
package main

import (
	"errors"
	"fmt"
	"runtime"
)

func try(f func()) {
	defer func() {
		r := recover()
		re, ok := r.(runtime.Error)
		_, isErr := r.(error)
		fmt.Println(ok, isErr, r)
		if ok {
			re.RuntimeError()
		}
	}()
	f()
}

func main() {
	var s []int
	var m map[string]int
	var p *struct{ x int }
	zero := 0
	var a any = "text"
	try(func() { _ = s[3] })
	try(func() { m["k"] = 1 })
	try(func() { _ = p.x })
	try(func() { _ = 1 / zero })
	try(func() { _ = a.(int) })
	try(func() { panic(errors.New("plain error")) })
	try(func() { panic("plain string") })
}

// Output:
// true true runtime error: index out of range [3] with length 0
// true true assignment to entry in nil map
// true true runtime error: invalid memory address or nil pointer dereference
// true true runtime error: integer divide by zero
// true true interface conversion: interface {} is string, not int
// false true plain error
// false false plain string
