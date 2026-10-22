// phase: 08
// divergence: D1
// lsan: cycle

// A sync.Cond whose Locker is a field of the struct that holds the Cond is a reference cycle: the
// struct is never freed (D1: Go's collector frees it). With the mutex in an allocation of its own
// nothing points back, and the struct is freed when the last pointer goes (the weak pointer turns
// nil at once). Real Go prints true twice, after a collection.
package main

import (
	"fmt"
	"sync"
	"weak"
)

type cyclic struct {
	mu   sync.Mutex
	cond *sync.Cond
}

type separate struct {
	mu   *sync.Mutex
	cond *sync.Cond
}

func main() {
	c := &cyclic{}
	c.cond = sync.NewCond(&c.mu)
	wc := weak.Make(c)
	c = nil
	fmt.Println("cycle freed:", wc.Value() == nil)

	s := &separate{mu: &sync.Mutex{}}
	s.cond = sync.NewCond(s.mu)
	ws := weak.Make(s)
	s.mu.Lock()
	s.mu.Unlock()
	s = nil
	fmt.Println("separate mutex freed:", ws.Value() == nil)
}

// Output:
// cycle freed: false
// separate mutex freed: true
