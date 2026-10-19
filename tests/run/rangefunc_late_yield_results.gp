// phase: 07

// A body that returns values sets the results of the function (boxed variables); a late yield
// after the function returned finds the loop finished and must not write the results.
package main

import "fmt"

type Res[T any] struct {
	Val T
	Ok  bool
}

var saved func(int) bool

func iter(yield func(int) bool) {
	saved = yield
	for i := 0; i < 5; i++ {
		if !yield(i) {
			return
		}
	}
}

func find[T any](want int, v T) (r Res[T], err error) {
	for x := range iter {
		if x == want {
			return Res[T]{v, true}, nil
		}
	}
	return Res[T]{}, fmt.Errorf("no %d", want)
}

func try(f func(int) bool) {
	defer func() { fmt.Println("late ->", recover()) }()
	f(3)
}

func main() {
	fmt.Println(find(2, "two"))
	try(saved)
	fmt.Println(find(9, 9.5))
	try(saved)
	fmt.Println(find(0, [2]int{4, 5}))
	try(saved)
}

// Output:
// {two true} <nil>
// late -> runtime error: range function continued iteration after whole loop exit
// {0 false} no 9
// late -> runtime error: range function continued iteration after whole loop exit
// {[4 5] true} <nil>
// late -> runtime error: range function continued iteration after whole loop exit
