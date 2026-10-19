// phase: 07

package main

import "fmt"

func Empty(yield func(int) bool) {
	defer fmt.Println("empty cleanup")
}

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("cleanup", n)
		for i := range n {
			fmt.Println("yielding", i)
			if !yield(i) {
				fmt.Println("told to stop at", i)
				return
			}
		}
		fmt.Println("finished normally")
	}
}

func main() {
	for x := range Empty {
		fmt.Println("never", x)
	}
	fmt.Println("after empty")
	for x := range Cnt(0) {
		fmt.Println("never", x)
	}
	for x := range Cnt(5) {
		if x == 1 {
			break
		}
	}
	fmt.Println("after break")
	for x := range Cnt(2) {
		_ = x
	}
	fmt.Println("after full")
	func() {
		for x := range Cnt(5) {
			if x == 2 {
				return
			}
		}
	}()
	fmt.Println("after return")
}

// Output:
// empty cleanup
// after empty
// finished normally
// cleanup 0
// yielding 0
// yielding 1
// told to stop at 1
// cleanup 5
// after break
// yielding 0
// yielding 1
// finished normally
// cleanup 2
// after full
// yielding 0
// yielding 1
// yielding 2
// told to stop at 2
// cleanup 5
// after return
