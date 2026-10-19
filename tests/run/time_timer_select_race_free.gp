// phase: 08

// Many goroutines wait on their own timers inside a select with a done channel; closing done
// wins for the long timers, the short ones fire first.
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	done := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	fired, cancelled := 0, 0
	for i := 0; i < 200; i++ {
		d := time.Hour
		if i%2 == 0 {
			d = time.Millisecond
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-t.C:
				mu.Lock()
				fired++
				mu.Unlock()
			case <-done:
				mu.Lock()
				cancelled++
				mu.Unlock()
			}
		}()
	}
	for {
		mu.Lock()
		f := fired
		mu.Unlock()
		if f == 100 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(done)
	wg.Wait()
	fmt.Println(fired, cancelled)
}

// Output:
// 100 100
