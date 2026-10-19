// phase: 07
// ExitCode: 2
// Stderr: ^panic: runtime error: range function continued iteration after whole loop exit
// Stderr: (?m)^goroutine 1 \[running\]:

// A generic iterator stores yield and calls it after the whole loop finished: "range function
// continued iteration after whole loop exit". The loop variable is a struct instance.
package main

import "fmt"

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

var saved func(Pair[string, int]) bool

func Items[K comparable, V any](m map[K]V) func(yield func(Pair[K, V]) bool) {
	return func(yield func(Pair[K, V]) bool) {
		for k, v := range m {
			if !yield(Pair[K, V]{k, v}) {
				return
			}
		}
		if f, ok := any(yield).(func(Pair[string, int]) bool); ok {
			saved = f
		}
	}
}

func main() {
	for p := range Items(map[string]int{"one": 1}) {
		fmt.Printf("%+v\n", p)
	}
	fmt.Println("loop over")
	saved(Pair[string, int]{"two", 2})
	fmt.Println("not reached")
}

// Output:
// {Key:one Val:1}
// loop over
