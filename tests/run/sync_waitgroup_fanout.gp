// phase: 08

// Fan-out and fan-in with a WaitGroup: each worker writes its own slot.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	results := make([]int, 20)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := 0
			for k := 1; k <= i*100; k++ {
				s += k
			}
			results[i] = s
		}()
	}
	wg.Wait()
	fmt.Println(results)
}

// Output:
// [0 5050 20100 45150 80200 125250 180300 245350 320400 405450 500500 605550 720600 845650 980700 1125750 1280800 1445850 1620900 1805950]
