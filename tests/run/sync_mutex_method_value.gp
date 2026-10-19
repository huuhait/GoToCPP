// phase: 08

// Method values and method expressions of a mutex are ordinary func values.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var mu sync.Mutex
	lock, unlock := mu.Lock, mu.Unlock
	try := mu.TryLock
	lock()
	fmt.Println("try while locked:", try())
	unlock()
	fmt.Println("try while free:", try())
	unlock()

	// A method expression takes the receiver as its first argument.
	l := (*sync.Mutex).Lock
	u := (*sync.Mutex).Unlock
	l(&mu)
	fmt.Println("locked:", !mu.TryLock())
	u(&mu)

	// Values that are passed around and called from other goroutines.
	n := 0
	var wg sync.WaitGroup
	critical := func(f func()) {
		lock()
		defer unlock()
		f()
	}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			critical(func() { n += i })
		}()
	}
	wg.Wait()
	fmt.Println("sum:", n)

	fs := []func(){mu.Lock, mu.Unlock}
	fs[0]()
	fs[1]()
	fmt.Println("done")
}

// Output:
// try while locked: false
// try while free: true
// locked: true
// sum: 1225
// done
