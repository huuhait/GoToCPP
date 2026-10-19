// phase: 08

package main

import "fmt"

func FromChan[T any](c <-chan T) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for {
			v, ok := <-c
			if !ok || !yield(v) {
				return
			}
		}
	}
}

func Producer(n int) <-chan int {
	c := make(chan int)
	go func() {
		defer close(c)
		for i := 0; i < n; i++ {
			c <- i
		}
	}()
	return c
}

// Pull runs the iterator in a goroutine and hands out its values one by one.
func Pull[T any](seq func(yield func(T) bool)) (next func() (T, bool), stop func()) {
	values := make(chan T)
	resume := make(chan bool)
	go func() {
		defer close(values)
		if !<-resume {
			return
		}
		seq(func(v T) bool {
			values <- v
			return <-resume
		})
	}()
	next = func() (T, bool) {
		resume <- true
		v, ok := <-values
		return v, ok
	}
	stop = func() {
		resume <- false
		<-values
	}
	return
}

func Count(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("count cleanup")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	for v := range FromChan(Producer(4)) {
		fmt.Println("got", v)
	}
	next, stop := Pull(Count(10))
	for range 3 {
		v, ok := next()
		fmt.Println(v, ok)
	}
	stop()
	fmt.Println("stopped")
}

// Output:
// got 0
// got 1
// got 2
// got 3
// 0 true
// 1 true
// 2 true
// count cleanup
// stopped
