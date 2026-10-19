// phase: 08

// Merging channels by setting a closed one to nil in the select: the merge ends when all are closed.
package main

import "fmt"

func produce(vals ...int) <-chan int {
	c := make(chan int)
	go func() {
		for _, v := range vals {
			c <- v
		}
		close(c)
	}()
	return c
}

func main() {
	a, b, c := produce(1, 2, 3), produce(10, 20), produce(100)
	sum, n := 0, 0
	for a != nil || b != nil || c != nil {
		select {
		case v, ok := <-a:
			if !ok {
				a = nil
				continue
			}
			sum += v
			n++
		case v, ok := <-b:
			if !ok {
				b = nil
				continue
			}
			sum += v
			n++
		case v, ok := <-c:
			if !ok {
				c = nil
				continue
			}
			sum += v
			n++
		}
	}
	fmt.Println(n, sum)
}

// Output:
// 6 136
