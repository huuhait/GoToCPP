// phase: 09
// runtime.Goexit inside a pulled sequence ends the coroutine and then the goroutine that called next, after its
// deferred calls (a goroutine here, so main goes on); a panic in the sequence comes out of next and can be recovered;
// the goroutine count goes back each time. The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it
// again).

package main

import (
	"fmt"
	"iter"
	"runtime"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	base := runtime.NumGoroutine()
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer fmt.Println("caller's deferred call", i)
			next, stop := iter.Pull(func(yield func(int) bool) {
				defer fmt.Println("sequence's deferred call", i)
				yield(i)
				if i%2 == 0 {
					runtime.Goexit()
				}
				yield(i + 10)
			})
			defer stop()
			for {
				v, ok := next()
				fmt.Println("value", i, v, ok)
				if !ok {
					break
				}
			}
			fmt.Println("not reached for even i? i =", i)
		}()
		wg.Wait()
	}
	last, same := -1, 0
	for i := 0; i < 50000 && same < 100; i++ { // goroutines that have called wg.Done may still be ending
		n := runtime.NumGoroutine()
		if n == last {
			same++
		} else {
			last, same = n, 0
		}
		runtime.Gosched()
		time.Sleep(100 * time.Microsecond)
	}
	fmt.Println("goroutines left:", last-base)
}

// Output:
// value 0 0 true
// sequence's deferred call 0
// caller's deferred call 0
// value 1 1 true
// value 1 11 true
// sequence's deferred call 1
// value 1 0 false
// not reached for even i? i = 1
// caller's deferred call 1
// value 2 2 true
// sequence's deferred call 2
// caller's deferred call 2
// goroutines left: 0
