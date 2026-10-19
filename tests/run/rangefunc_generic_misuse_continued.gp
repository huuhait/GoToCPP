// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function continued iteration after function for loop body returned false
// Stderr: (?m)^goroutine 1 \[running\]:

// A generic iterator keeps calling yield after the body asked to stop (break): the fourth state of
// Go's rewrite panics with "range function continued iteration after function for loop body returned
// false", from inside the iterator, which runs for a generic Seq[T] with two type arguments.
package main

import "fmt"

type Seq[T any] func(yield func(T) bool)

func Of[T any](xs ...T) Seq[T] {
	return func(yield func(T) bool) {
		for _, x := range xs {
			yield(x) // forgets to check the result
		}
	}
}

func main() {
	for x := range Of("a", "b", "c") {
		fmt.Println("got", x)
		if x == "a" {
			break
		}
	}
	fmt.Println("not reached")
}

// Output:
// got a
