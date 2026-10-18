// phase: 08

// With one producer, a range loop sees the values in the order they were sent, through unbuffered
// and buffered channels of every size.
package main

import "fmt"

func main() {
	for _, size := range []int{0, 1, 3, 100} {
		c := make(chan int, size)
		go func() {
			for i := 0; i < 1000; i++ {
				c <- i
			}
			close(c)
		}()
		next := 0
		ordered := true
		for v := range c {
			if v != next {
				ordered = false
			}
			next++
		}
		fmt.Println(size, next, ordered)
	}
}

// Output:
// 0 1000 true
// 1 1000 true
// 3 1000 true
// 100 1000 true
