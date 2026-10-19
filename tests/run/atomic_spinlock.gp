// phase: 08

// A spin lock from CompareAndSwap protects a plain counter; a ticket lock from AddUint32 serves in order.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

type spin struct{ state int32 }

func (s *spin) Lock() {
	for !atomic.CompareAndSwapInt32(&s.state, 0, 1) {
		runtime.Gosched()
	}
}
func (s *spin) Unlock() { atomic.StoreInt32(&s.state, 0) }

type ticket struct{ next, serving atomic.Uint32 }

func (t *ticket) Lock() {
	me := t.next.Add(1) - 1
	for t.serving.Load() != me {
		runtime.Gosched()
	}
}
func (t *ticket) Unlock() { t.serving.Add(1) }

func run(l sync.Locker) int {
	var wg sync.WaitGroup
	n := 0
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				l.Lock()
				n++
				l.Unlock()
			}
		}()
	}
	wg.Wait()
	return n
}

func main() {
	fmt.Println(run(&spin{}))
	t := &ticket{}
	fmt.Println(run(t), t.next.Load(), t.serving.Load())
}

// Output:
// 4000
// 4000 4000 4000
