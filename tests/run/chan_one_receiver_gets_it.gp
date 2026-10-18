// phase: 08

// Each value goes to exactly one of several receivers: the counts add up and nothing is lost or
// delivered twice.
package main

import "fmt"

func main() {
	c := make(chan int)
	counts := make(chan [2]int)
	const receivers, values = 5, 1000
	for r := 0; r < receivers; r++ {
		go func() {
			n, sum := 0, 0
			for {
				v, ok := <-c
				if !ok {
					break
				}
				n++
				sum += v
			}
			counts <- [2]int{n, sum}
		}()
	}
	for i := 1; i <= values; i++ {
		c <- i
	}
	close(c)
	n, sum := 0, 0
	for r := 0; r < receivers; r++ {
		x := <-counts
		n += x[0]
		sum += x[1]
	}
	fmt.Println(n, sum)
}

// Output:
// 1000 500500
