// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function continued iteration after loop body panic
// Stderr: (?m)^goroutine 1 \[running\]:

// The body panics, the generic iterator recovers that panic and calls yield again: Go panics with
// "range function continued iteration after loop body panic" (uncaught here: exit 2).
package main

import "fmt"

type Box[T any] struct{ v T }

func (b Box[T]) All(yield func(T) bool) {
	func() {
		defer func() { fmt.Println("iterator recovered:", recover()) }()
		yield(b.v)
	}()
	fmt.Println("calling yield again")
	yield(b.v)
	fmt.Println("not reached")
}

func main() {
	for x := range (Box[[2]string]{[2]string{"p", "q"}}).All {
		fmt.Println("body", x)
		panic("first panic")
	}
}

// Output:
// body [p q]
// iterator recovered: first panic
// calling yield again
