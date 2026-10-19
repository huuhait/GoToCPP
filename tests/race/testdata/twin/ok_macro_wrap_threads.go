package main

import (
	"fmt"
	"sync"
)

// The wrap macros are functions that get the code as a func value; the functions keep their results
// in named variables that the func literal sets.

func InThread(body func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		body()
	}()
	<-done
}

func Once(o *sync.Once, body func()) {
	o.Do(func() { body() })
}

func Recover(on *bool, body func()) {
	defer func() {
		if r := recover(); r != nil {
			*on = true
		}
	}()
	body()
}

var (
	setupOnce  sync.Once
	setupCalls int
	mu         sync.Mutex
	recovered  = map[int]bool{}
)

func setup() {
	Once(&setupOnce, func() {
		func() {
			setupCalls++
		}()
	})
}

func square(n int) (gp_r0 int) {
	InThread(func() {
		gp_r0 = func() (r int) {
			r = n * n
			return r
		}()
	})
	return
}

func risky(i int) (out int) {
	var hit bool
	out = guarded(i, &hit)
	mu.Lock()
	recovered[i] = hit
	mu.Unlock()
	return out
}

func guarded(i int, hit *bool) (gp_r0 int) {
	Recover(hit, func() {
		gp_r0 = func() int {
			if i%2 == 1 {
				panic("odd")
			}
			return i * 10
		}()
	})
	return
}

func main() {
	var wg sync.WaitGroup
	results := make([]int, 4)
	for g := range 4 {
		wg.Go(func() {
			setup()
			results[g] = square(g+1) + risky(g)
		})
	}
	wg.Wait()
	fmt.Println(setupCalls, results, recovered)
}
