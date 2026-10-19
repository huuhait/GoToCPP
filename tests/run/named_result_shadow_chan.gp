// phase: 08

// A named result hidden by a variable of an inner scope in a function with a defer, with channels
// and goroutines: the variables of a select case and of a for range over a channel, a function
// that runs in a goroutine, and a deferred call that waits for one.
package main

import "fmt"

func selectInit(ch chan int) (v int, ok bool) {
	defer func() { ok = !ok }()
	select {
	case v, ok := <-ch:
		return v + 1, ok
	}
}

func selectDefault(ch chan int) (v int, s string) {
	defer func() { s += "." }()
	select {
	case v := <-ch:
		return v * 2, "got"
	default:
		return -1, "none"
	}
}

func rangeChan(ch chan int) (i int, sum int) {
	defer func() { sum *= 10 }()
	for i := range ch {
		sum += i
		if i == 3 {
			return i, sum
		}
	}
	return -1, sum
}

func goroutine() (i int, s string) {
	done := make(chan bool)
	defer func() { <-done }()
	go func() {
		defer func() { done <- true }()
	}()
	for i, s := range []string{"x", "y"} {
		if s == "y" {
			return i, s
		}
	}
	return 0, ""
}

func inGoroutine() (int, string) {
	out := make(chan string)
	go func() {
		f := func() (i int, s string) {
			defer func() { s += "!" }()
			for i, s := range []string{"a", "b", "c"} {
				if s == "c" {
					return i, s
				}
			}
			return -1, ""
		}
		i, s := f()
		out <- fmt.Sprint(i, s)
	}()
	return 1, <-out
}

func main() {
	ch := make(chan int, 4)
	ch <- 9
	fmt.Println(selectInit(ch))
	close(ch)
	fmt.Println(selectInit(ch))
	c2 := make(chan int, 1)
	fmt.Println(selectDefault(c2))
	c2 <- 21
	fmt.Println(selectDefault(c2))
	c3 := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		c3 <- i
	}
	fmt.Println(rangeChan(c3))
	fmt.Println(goroutine())
	fmt.Println(inGoroutine())
}

// Output:
// 10 false
// 1 true
// -1 none.
// 42 got.
// 3 60
// 1 y
// 1 2c!
