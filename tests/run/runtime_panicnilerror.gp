// phase: 08

// panic(nil) panics with a *runtime.PanicNilError (Go 1.21 and later), which is a runtime.Error.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	defer func() {
		r := recover()
		e, ok := r.(*runtime.PanicNilError)
		fmt.Println(ok, r != nil)
		fmt.Println(e.Error())
		_, isRT := r.(runtime.Error)
		fmt.Println(isRT)
	}()
	panic(nil)
}

// Output:
// true true
// panic called with nil argument
// true
