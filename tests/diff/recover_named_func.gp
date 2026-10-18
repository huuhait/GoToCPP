// phase: 05

package main

import "fmt"

var count int

// handler is the deferred function itself and calls recover directly.
func handler() {
	if r := recover(); r != nil {
		count++
		fmt.Println("handler recovered:", r)
	}
}

func work(n int) {
	defer handler()
	if n > 0 {
		panic(fmt.Sprint("work ", n))
	}
	fmt.Println("work ok")
}

func main() {
	work(0)
	work(1)
	work(2)
	fmt.Println("count", count)
}
