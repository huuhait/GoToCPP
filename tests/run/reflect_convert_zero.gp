// phase: 09

// Value.Convert of a zero Value or to a nil Type reads through a nil pointer in Go (step 9.3
// lows): the panic is a runtime error with the nil dereference text, not a ValueError or a
// failed type assertion.
package main

import (
	"fmt"
	"reflect"
	"runtime"
)

func try(name string, f func()) {
	defer func() {
		r := recover()
		err, isError := r.(error)
		_, isRuntime := r.(runtime.Error)
		fmt.Println(name, isError, isRuntime)
		if isError {
			fmt.Println(err.Error())
		}
	}()
	f()
}

func main() {
	try("zero value", func() { reflect.Value{}.Convert(reflect.TypeOf(1)) })
	try("nil type", func() { reflect.ValueOf(1).Convert(nil) })
	try("both", func() { reflect.Value{}.Convert(nil) })
	fmt.Println(reflect.ValueOf(65).Convert(reflect.TypeOf("")).String())
	try("not convertible", func() { reflect.ValueOf("x").Convert(reflect.TypeOf(1)) })
}

// Output:
// zero value true true
// runtime error: invalid memory address or nil pointer dereference
// nil type true true
// runtime error: invalid memory address or nil pointer dereference
// both true true
// runtime error: invalid memory address or nil pointer dereference
// A
// not convertible false false
