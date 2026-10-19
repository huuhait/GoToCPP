// phase: 07

package main

import "fmt"

var calls int

func make1(n int) func(yield func(int) bool) {
	calls++
	fmt.Println("make1 called", calls)
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	for x := range make1(3) {
		fmt.Println(x)
	}
	for x := range make1(2) {
		for y := range make1(2) {
			fmt.Println(x, y)
		}
	}
	f := make1(1)
	for range f {
	}
	for range f {
	}
	fmt.Println("calls", calls)
	var seq func(yield func(int) bool)
	seq = func(yield func(int) bool) {
		// reassigning the variable while the loop runs does not change the loop
		seq = nil
		yield(1)
		yield(2)
	}
	for x := range seq {
		fmt.Println("seq", x, seq == nil)
	}
}

// Output:
// make1 called 1
// 0
// 1
// 2
// make1 called 2
// make1 called 3
// 0 0
// 0 1
// make1 called 4
// 1 0
// 1 1
// make1 called 5
// calls 5
// seq 1 true
// seq 2 true
