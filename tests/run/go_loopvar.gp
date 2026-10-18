// phase: 08

// Go 1.22: each iteration has its own copy of the loop variable, so goroutines started in a loop
// see their own value.
package main

import "fmt"

func main() {
	res := make(chan int, 10)
	for i := 0; i < 5; i++ {
		go func() { res <- i * i }()
	}
	sum := 0
	for k := 0; k < 5; k++ {
		sum += <-res
	}
	fmt.Println(sum) // 0+1+4+9+16

	words := []string{"a", "b", "c"}
	out := make(chan string, 3)
	for _, w := range words {
		go func() { out <- w + w }()
	}
	seen := map[string]bool{}
	for range words {
		seen[<-out] = true
	}
	fmt.Println(seen["aa"], seen["bb"], seen["cc"], len(seen))

	// a loop variable that the body changes
	total := make(chan int, 4)
	for i := 0; i < 8; i++ {
		done := make(chan bool)
		go func() { total <- i; done <- true }()
		<-done // the goroutine has read this iteration's i before the body changes it
		i++
	}
	s := 0
	for k := 0; k < 4; k++ {
		s += <-total
	}
	fmt.Println(s)
}

// Output:
// 30
// true true true 3
// 12
