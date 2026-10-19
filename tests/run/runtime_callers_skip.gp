// phase: 08

// Callers' skip: 0 is Callers itself, 1 its caller.
package main

import (
	"fmt"
	"runtime"
)

func first(skip int) string {
	pcs := make([]uintptr, 1)
	if runtime.Callers(skip, pcs) == 0 {
		return "none"
	}
	f, _ := runtime.CallersFrames(pcs).Next()
	return f.Function
}

func f() string { return first(1) }
func g() string { return first(2) }
func h() string { return g() }

func main() {
	fmt.Println(first(0))
	fmt.Println(f())
	fmt.Println(h())
	fmt.Println(first(500))
}

// Output:
// runtime.Callers
// main.first
// main.g
// none
