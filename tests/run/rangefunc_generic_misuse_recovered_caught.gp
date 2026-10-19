// phase: 08

// A generic iterator recovers every panic of the body and returns normally: each loop ends with
// "range function recovered a loop body panic and did not resume panicking", a runtime.Error that
// the caller recovers.
package main

import (
	"fmt"
	"runtime"
)

func Keep[T any](xs []T) func(func(T) bool) {
	return func(yield func(T) bool) {
		for _, x := range xs {
			func() {
				defer func() { recover() }()
				yield(x)
			}()
		}
	}
}

func try(name string, f func()) {
	defer func() {
		r := recover()
		_, isRuntime := r.(runtime.Error)
		fmt.Printf("%s: %v (runtime.Error %v)\n", name, r, isRuntime)
	}()
	f()
}

func main() {
	try("int8", func() {
		for x := range Keep([]int8{1, 2, 3}) {
			fmt.Println("body", x)
			panic("boom")
		}
	})
	try("strings", func() {
		for s := range Keep([]string{"a", "b"}) {
			fmt.Println("body", s)
			panic(s)
		}
	})
}

// Output:
// body 1
// int8: runtime error: range function recovered a loop body panic and did not resume panicking (runtime.Error true)
// body a
// strings: runtime error: range function recovered a loop body panic and did not resume panicking (runtime.Error true)
