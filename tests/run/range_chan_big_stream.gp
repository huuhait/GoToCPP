// phase: 08

// 100000 values through a range loop, unbuffered and buffered; sums are checked.
package main

import "fmt"

func main() {
	const n = 100000
	for _, size := range []int{0, 64} {
		c := make(chan int, size)
		go func() {
			for i := 1; i <= n; i++ {
				c <- i
			}
			close(c)
		}()
		sum := 0
		for v := range c {
			sum += v
		}
		fmt.Println(size, sum == n*(n+1)/2)
	}
}

// Output:
// 0 true
// 64 true
