package main

import "fmt"

func worker(id int, done chan<- int) { done <- id * id }

func main() {
	done := make(chan int)
	for i := 1; i <= 3; i++ {
		go worker(i, done)
	}
	sum := 0
	for range 3 {
		sum += <-done
	}
	fmt.Println(sum)
}
