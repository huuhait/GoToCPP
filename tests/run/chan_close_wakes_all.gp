// phase: 08

// Closing a channel wakes every receiver blocked on it, with the zero value and ok == false;
// a value sent before the close is delivered to exactly one receiver first.
package main

import (
	"fmt"
	"sync"
)

func main() {
	c := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	zero, got := 0, 0
	blocked := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blocked <- true
			v, ok := <-c
			mu.Lock()
			defer mu.Unlock()
			if ok {
				got += v
			} else if v == 0 {
				zero++
			}
		}()
	}
	for i := 0; i < 20; i++ {
		<-blocked
	}
	c <- 7
	close(c)
	wg.Wait()
	fmt.Println(got, zero)
	v, ok := <-c
	fmt.Println(v, ok)
}

// Output:
// 7 19
// 0 false
