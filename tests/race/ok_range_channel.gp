// phase: 08

// A goroutine that ranges over a channel reads what the producer wrote before each send.
package main

import "fmt"

type Batch struct{ vals [4]int }

func main() {
	ch := make(chan *Batch, 2)
	sum := make(chan int)
	go func() {
		t := 0
		for b := range ch {
			for _, v := range b.vals {
				t += v
			}
		}
		sum <- t
	}()
	for i := 0; i < 10; i++ {
		b := &Batch{}
		for j := range b.vals {
			b.vals[j] = i
		}
		ch <- b
	}
	close(ch)
	fmt.Println(<-sum)
}

// Output:
// 180
