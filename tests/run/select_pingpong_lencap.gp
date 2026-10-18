// phase: 08

// select ping-pong between two goroutines, with len and cap of the channels used in the cases.
package main

import "fmt"

func main() {
	ping := make(chan int)
	pong := make(chan int, 1)
	quit := make(chan bool)
	go func() {
		for {
			select {
			case v := <-ping:
				pong <- v + 1
			case <-quit:
				return
			}
		}
	}()
	v := 0
	for i := 0; i < 2000; i++ {
		select {
		case ping <- v:
		}
		select {
		case v = <-pong:
		}
	}
	quit <- true
	fmt.Println(v, len(ping), cap(ping), len(pong), cap(pong))

	buf := make(chan int, 4)
	for i := 0; ; i++ {
		select {
		case buf <- i:
			continue
		default:
		}
		break
	}
	fmt.Println(len(buf), cap(buf))
	for len(buf) > 0 {
		select {
		case x := <-buf:
			fmt.Println("item", x)
		}
	}
}

// Output:
// 2000 0 0 0 1
// 4 4
// item 0
// item 1
// item 2
// item 3
