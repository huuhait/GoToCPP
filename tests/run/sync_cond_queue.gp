// phase: 08
// lsan: cycle

// Producers and consumers share a queue guarded by a Mutex and a Cond: every item is consumed once.
//
// (The Cond's Locker is a pointer into the struct that holds the Cond: a reference cycle, which
// reference counting never frees, divergence D1. LeakSanitizer is off for this test.)
package main

import (
	"fmt"
	"sync"
)

type Queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []int
	closed bool
}

func NewQueue() *Queue {
	// The room for every item that the program puts (3 producers x 1000): the queue never grows, so the
	// memory that the leaked queue holds (see tools/sanitize/leaks.txt) is the same on every run, whatever
	// the order of the goroutines (a queue that grows by append holds 200 to 800 bytes, depending on it).
	q := &Queue{items: make([]int, 0, 3000)}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *Queue) Put(v int) {
	q.mu.Lock()
	q.items = append(q.items, v)
	q.mu.Unlock()
	q.cond.Signal()
}

func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

func (q *Queue) Get() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return 0, false
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v, true
}

func main() {
	q := NewQueue()
	var consumers sync.WaitGroup
	var mu sync.Mutex
	sum, count := 0, 0
	for c := 0; c < 4; c++ {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				v, ok := q.Get()
				if !ok {
					return
				}
				mu.Lock()
				sum += v
				count++
				mu.Unlock()
			}
		}()
	}
	var producers sync.WaitGroup
	for p := 0; p < 3; p++ {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for i := 1; i <= 1000; i++ {
				q.Put(p*1000 + i)
			}
		}()
	}
	producers.Wait()
	q.Close()
	consumers.Wait()
	fmt.Println(count, sum)
}

// Output:
// 3000 4501500
