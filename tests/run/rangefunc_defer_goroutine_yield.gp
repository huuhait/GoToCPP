// phase: 08

package main

import "fmt"

// The iterator runs the loop body on another goroutine: a defer in the body still belongs to
// the function that has the loop.
func OnGoroutine(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		done := make(chan bool)
		go func() {
			defer close(done)
			for i := range n {
				if !yield(i) {
					return
				}
			}
		}()
		<-done
	}
}

func f() {
	defer fmt.Println("f deferred")
	for x := range OnGoroutine(3) {
		defer fmt.Println("body defer", x)
		fmt.Println("body", x)
	}
	fmt.Println("loop over")
}

func g() int {
	for x := range OnGoroutine(10) {
		if x == 4 {
			return x * 2
		}
	}
	return -1
}

func main() {
	f()
	fmt.Println(g())
}

// Output:
// body 0
// body 1
// body 2
// loop over
// body defer 2
// body defer 1
// body defer 0
// f deferred
// 8
