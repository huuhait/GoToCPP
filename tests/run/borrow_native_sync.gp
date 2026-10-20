// phase: 08

// A callee without an analysed body (sync.Mutex.Unlock, sync.WaitGroup.Wait) runs other goroutines, which drop the package variable that the caller lent: the call must not treat it as pure (ADR-086).

package main

import (
	"fmt"
	"sync"
)

type T struct {
	n    int
	name string
}

var g *T
var mu sync.Mutex
var wg sync.WaitGroup

func junk() int {
	t := 0
	for i := 0; i < 100; i++ {
		x := &T{n: i, name: "junkjunkjunk"}
		t += x.n
	}
	return t
}

// release lets the other goroutine run (Unlock), waits until it has dropped g (Wait), then reads t.
func (t *T) release() int {
	mu.Unlock()
	wg.Wait()
	return t.n + len(t.name)
}

func rel(t *T) int {
	mu.Unlock()
	wg.Wait()
	return t.n + len(t.name)
}

func main() {
	for round := 0; round < 4; round++ {
		g = &T{n: round, name: "object-with-a-long-name"}
		mu.Lock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			g = nil
			junk()
			mu.Unlock()
		}()
		if round%2 == 0 {
			fmt.Println(g.release())
		} else {
			fmt.Println(rel(g))
		}
	}
}

// Output:
// 23
// 24
// 25
// 26
