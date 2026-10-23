package lib

import "fmt"

// Count prints the element 1 of a, which the caller copied at its defer or go statement, and n.
func Count(tag string, a [1 << 20]int, n int) {
	fmt.Println(tag, a[1], n)
}

// Send sends the element 1 of a and n.
func Send(a [1 << 20]int, n int, done chan int) {
	done <- a[1]*10 + n
}
