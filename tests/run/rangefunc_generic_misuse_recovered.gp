// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function recovered a loop body panic and did not resume panicking
// Stderr: (?m)^goroutine 1 \[running\]:

// The iterator recovers the body's panic and returns normally: "range function recovered a loop
// body panic and did not resume panicking" (state MISSING_PANIC), from a generic iterator over a
// defined type, uncaught; the deferred call of main still runs.
package main

import "fmt"

type ID int8

func IDs[T ~int8](from, to T) func(func(T) bool) {
	return func(yield func(T) bool) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("iterator swallowed:", r)
			}
		}()
		for i := from; i < to; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	defer fmt.Println("main deferred")
	for id := range IDs(ID(1), ID(4)) {
		fmt.Println("id", id)
		if id == 2 {
			panic(fmt.Sprintf("body panic %d", id))
		}
	}
	fmt.Println("not reached")
}

// Output:
// id 1
// id 2
// iterator swallowed: body panic 2
// main deferred
