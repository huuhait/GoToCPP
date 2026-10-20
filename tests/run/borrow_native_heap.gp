// phase: 08

// A callee that runs other goroutines through the synchronization natives (Unlock, Wait) is not
// pure, also when the argument is loaded from a heap holder (h.p). (ADR-086, section 4)

package main

import (
	"fmt"
	"sync"
)

type T struct {
	n    int
	name string
}

type Holder struct{ p *T }

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
		// The slice element and its field are initialized, never assigned or address-taken:
		// &Holder{...} would mark the field hot and mask an incorrectly pure callee.
		// Dropping the boxed local h releases the backing holder AND its only owning
		// pointer to T, after release/rel unlocks mu.
		h := []Holder{{p: &T{n: round, name: "object-with-a-long-name"}}}
		mu.Lock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			h = nil
			junk()
			mu.Unlock()
		}()
		if round%2 == 0 {
			fmt.Println(h[0].p.release())
		} else {
			fmt.Println(rel(h[0].p))
		}
	}
}

// Output:
// 23
// 24
// 25
// 26
