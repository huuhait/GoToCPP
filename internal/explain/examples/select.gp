package main

import "fmt"

func main() {
	a, b := make(chan int, 1), make(chan string, 1)
	a <- 1
	select {
	case v := <-a:
		fmt.Println("a", v)
	case s := <-b:
		fmt.Println("b", s)
	default:
		fmt.Println("none")
	}
}
