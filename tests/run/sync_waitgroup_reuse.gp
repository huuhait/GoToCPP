// phase: 08

// A WaitGroup can be reused once the previous Wait has returned: ten rounds on one value.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

func main() {
	var wg sync.WaitGroup
	var n atomic.Int64
	for round := 1; round <= 10; round++ {
		for i := 0; i < round*3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n.Add(1)
			}()
		}
		wg.Wait()
		fmt.Println("round", round, "total", n.Load())
	}
}

// Output:
// round 1 total 3
// round 2 total 9
// round 3 total 18
// round 4 total 30
// round 5 total 45
// round 6 total 63
// round 7 total 84
// round 8 total 108
// round 9 total 135
// round 10 total 165
