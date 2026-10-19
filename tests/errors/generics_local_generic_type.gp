// phase: 07

// Go accepts a generic type declared inside a function (gc compiles this file); gocpp has types declared
// inside functions, also inside generic functions, and generic types at package level, but not both at once:
// the checker says so at the declaration instead of the compiler stopping inside (a stack trace of go/types).
package main

import "fmt"

func H() any {
	type Wrap[U any] struct { // ERROR "generic types declared inside functions are not supported in gocpp"
		u U
	}
	return Wrap[int]{1}
}

func G[T any](v T) any {
	type Pair[U any] struct { // (one error for each feature in a file, at its first use)
		t T
		u U
	}
	return Pair[T]{v, v}
}

func main() {
	fmt.Println(H(), G("s"))
}
