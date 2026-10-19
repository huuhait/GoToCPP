package main

import "fmt"

func main() {
	ch := make(chan int, 2)
	go func() {
		for i := 0; i < 3; i++ {
			ch <- i
		}
		close(ch)
	}()
	for v := range ch {
		fmt.Println(v)
	}
	v, ok := <-ch
	fmt.Println(v, ok)
}
