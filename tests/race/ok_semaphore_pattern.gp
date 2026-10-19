// phase: 08

// The k-th receive of a buffered channel is ordered before the (k+C)-th send completes.
package main

import "fmt"

var slots [3]int

func main() {
	sem := make(chan int, 3)
	for i := 0; i < 3; i++ {
		sem <- i
	}
	done := make(chan bool)
	total := 0
	for g := 0; g < 9; g++ {
		go func() {
			s := <-sem
			slots[s] += 10
			sem <- s
			done <- true
		}()
	}
	for g := 0; g < 9; g++ {
		<-done
	}
	for _, v := range slots {
		total += v
	}
	fmt.Println(total)
}

// Output:
// 90
