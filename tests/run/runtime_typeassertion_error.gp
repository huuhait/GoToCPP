// phase: 08

// A failed type assertion panics with a *runtime.TypeAssertionError, and the program can name
// its type.
package main

import (
	"fmt"
	"runtime"
)

type Shape interface{ Area() int }

func main() {
	defer func() {
		r := recover()
		e, ok := r.(*runtime.TypeAssertionError)
		fmt.Println(ok)
		fmt.Println(e.Error())
		var re runtime.Error = e
		re.RuntimeError()
	}()
	var x any = 3.5
	_ = x.(Shape)
}

// Output:
// true
// interface conversion: float64 is not main.Shape: missing method Area
