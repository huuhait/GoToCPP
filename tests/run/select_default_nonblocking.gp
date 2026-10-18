// phase: 08

// The non-blocking idioms: a select with one case and a default.
package main

import "fmt"

func tryRecv(c chan int) (int, bool) {
	select {
	case v := <-c:
		return v, true
	default:
		return 0, false
	}
}

func trySend(c chan int, v int) bool {
	select {
	case c <- v:
		return true
	default:
		return false
	}
}

func main() {
	c := make(chan int, 2)
	fmt.Println(tryRecv(c))
	fmt.Println(trySend(c, 1), trySend(c, 2), trySend(c, 3))
	fmt.Println(len(c), cap(c))
	fmt.Println(tryRecv(c))
	fmt.Println(tryRecv(c))
	fmt.Println(tryRecv(c))

	u := make(chan int) // unbuffered, nobody waits
	fmt.Println(trySend(u, 1))
	fmt.Println(tryRecv(u))

	var nilc chan int
	fmt.Println(trySend(nilc, 1))
	fmt.Println(tryRecv(nilc))

	// An unbuffered send succeeds when a receiver is already waiting.
	done := make(chan bool)
	got := make(chan int)
	go func() {
		got <- <-u
		done <- true
	}()
	for !trySend(u, 99) {
	}
	fmt.Println(<-got)
	<-done
}

// Output:
// 0 false
// true true false
// 2 2
// 1 true
// 2 true
// 0 false
// false
// 0 false
// false
// 0 false
// 99
