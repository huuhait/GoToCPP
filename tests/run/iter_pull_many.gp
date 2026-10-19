// phase: 09
// iter.Pull in bulk: 10000 pulls one after the other (half stopped early, half run to the end), and sixty-four
// goroutines that pull at the same time, handing the pulled values over through shared state under a mutex. It also
// runs with -race and -asan, where finished goroutine threads are kept for a while. The expected output comes from
// real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"fmt"
	"iter"
	"runtime"
	"sync"
)

func count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	sum := 0
	for i := 0; i < 10000; i++ {
		next, stop := iter.Pull(count(8))
		for j := 0; j < 4+i%3; j++ {
			v, _ := next()
			sum += v
		}
		if i%2 == 0 {
			stop()
		} else {
			for {
				v, ok := next()
				if !ok {
					break
				}
				sum += v
			}
			stop()
		}
	}
	fmt.Println("sequential:", sum)

	var mu sync.Mutex
	var wg sync.WaitGroup
	total, n := 0, 0
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < 50; r++ {
				next, stop := iter.Pull2(func(yield func(int, string) bool) {
					for i := 0; i < 5+g%4; i++ {
						if !yield(i, "x") {
							return
						}
					}
				})
				for {
					k, s, ok := next()
					if !ok {
						break
					}
					mu.Lock()
					total += k + len(s)
					n++
					mu.Unlock()
				}
				stop()
			}
		}()
	}
	wg.Wait()
	fmt.Println("concurrent:", total, n)
	last, same := -1, 0
	for i := 0; i < 50000 && same < 100; i++ {
		c := runtime.NumGoroutine()
		if c == last {
			same++
		} else {
			last, same = c, 0
		}
		runtime.Gosched()
	}
	fmt.Println("goroutines:", last)
}

// Output:
// sequential: 191667
// concurrent: 80000 20800
// goroutines: 1
