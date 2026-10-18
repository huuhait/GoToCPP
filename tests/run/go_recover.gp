// phase: 08

// A goroutine recovers its own panic with a deferred function; the program goes on.
package main

import "fmt"

func risky(n int, out chan<- string) {
	defer func() {
		if r := recover(); r != nil {
			out <- fmt.Sprint("recovered: ", r)
		}
	}()
	arr := []int{1, 2, 3}
	out <- fmt.Sprint("value ", arr[n])
}

func main() {
	out := make(chan string)
	for _, n := range []int{1, 5} {
		go risky(n, out)
		fmt.Println(<-out)
	}
	done := make(chan bool)
	go func() {
		defer close(done)
		defer func() { fmt.Println("recover:", recover()) }()
		var m map[string]int
		m["x"] = 1
	}()
	<-done
	// a panic with a custom value, re-panicked and recovered by an outer deferred call
	go func() {
		defer func() { done2 <- fmt.Sprint("outer got ", recover()) }()
		defer func() {
			r := recover()
			panic(fmt.Sprint("re-panic of ", r))
		}()
		panic("first")
	}()
	fmt.Println(<-done2)
}

var done2 = make(chan string)

// Output:
// value 2
// recovered: runtime error: index out of range [5] with length 3
// recover: assignment to entry in nil map
// outer got re-panic of first
