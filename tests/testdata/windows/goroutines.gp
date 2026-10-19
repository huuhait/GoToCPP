// phase: 08

package main

import "fmt"

func worker(id int, out chan<- int) {
	sum := 0
	for i := 1; i <= 1000; i++ {
		sum += i * id
	}
	out <- sum
}

func main() {
	out := make(chan int)
	for id := 1; id <= 8; id++ {
		go worker(id, out)
	}
	total := 0
	for range 8 {
		total += <-out
	}
	fmt.Println("total", total)
}
