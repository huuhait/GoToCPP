// phase: 08

// TryLock never blocks: it fails while the mutex is held, by this goroutine or another one.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	fmt.Println(mu.TryLock(), mu.TryLock())
	mu.Unlock()
	fmt.Println(mu.TryLock())
	mu.Unlock()

	// Another goroutine holds the lock: TryLock fails until it lets go.
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		mu.Lock()
		close(held)
		<-release
		mu.Unlock()
		close(done)
	}()
	<-held
	fmt.Println("while held:", mu.TryLock())
	close(release)
	<-done
	fmt.Println("after release:", mu.TryLock())
	mu.Unlock()

	// A busy loop of TryLock calls from many goroutines: the winners are counted exactly.
	var wg sync.WaitGroup
	var win sync.Mutex
	total := 0
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := 0
			for got < 1000 {
				if mu.TryLock() {
					got++
					mu.Unlock()
				}
			}
			win.Lock()
			total += got
			win.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("total:", total)
}

// Output:
// true false
// true
// while held: false
// after release: true
// total: 8000
