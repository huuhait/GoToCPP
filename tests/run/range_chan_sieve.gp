// phase: 08

// The concurrent prime sieve of the Go tour: a chain of goroutines, each filtering multiples of a
// prime, with range loops.
package main

import "fmt"

func generate(ch chan<- int) {
	for i := 2; ; i++ {
		ch <- i
	}
}

func filter(in <-chan int, out chan<- int, prime int) {
	for i := range in {
		if i%prime != 0 {
			out <- i
		}
	}
}

func main() {
	ch := make(chan int)
	go generate(ch)
	for i := 0; i < 25; i++ {
		prime := <-ch
		fmt.Println(prime)
		ch1 := make(chan int)
		go filter(ch, ch1, prime)
		ch = ch1
	}
}

// Output:
// 2
// 3
// 5
// 7
// 11
// 13
// 17
// 19
// 23
// 29
// 31
// 37
// 41
// 43
// 47
// 53
// 59
// 61
// 67
// 71
// 73
// 79
// 83
// 89
// 97
