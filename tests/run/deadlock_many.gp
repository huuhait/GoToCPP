// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[sync\.WaitGroup\.Wait\]:\n
// Stderr: (?s)(?:\ngoroutine \d+ \[chan receive\]:\n.*){3}

// Ten goroutines, three of them leaked on a channel nobody sends on: the report lists all that are
// left.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	leak := make(chan int)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			if i%4 == 0 {
				<-leak // 0, 4, 8: never done
				wg.Done()
				return
			}
			wg.Done()
		}()
	}
	fmt.Println("waiting")
	wg.Wait()
	fmt.Println("not reached")
}

// Output:
// waiting
