// phase: 08

// A Mutex is two words: a copy made before its first use is a fresh mutex, and a copy of a locked
// mutex is locked (go vet's copylocks complains; the program is valid).
package main

import (
	"fmt"
	"sync"
)

type Box struct {
	mu sync.Mutex
	v  int
}

func main() {
	proto := Box{v: 7}
	a, b := proto, proto
	a.mu.Lock()
	fmt.Println("a locked, b free:", !a.mu.TryLock(), b.mu.TryLock())
	b.mu.Unlock()
	a.mu.Unlock()

	boxes := make([]Box, 3)
	for i := range boxes {
		boxes[i] = proto
		boxes[i].v += i
	}
	for i := range boxes {
		boxes[i].mu.Lock()
	}
	for i := range boxes {
		boxes[i].mu.Unlock()
	}
	fmt.Println(boxes[0].v, boxes[1].v, boxes[2].v)

	// A copy of a locked mutex is locked.
	a.mu.Lock()
	c := a
	fmt.Println("copy is locked:", !c.mu.TryLock())
	c.mu.Unlock()
	fmt.Println("copy now free:", c.mu.TryLock())
	fmt.Println("original still locked:", !a.mu.TryLock())
	a.mu.Unlock()

	// Mutexes compare as their state does.
	var m1, m2 sync.Mutex
	fmt.Println(m1 == m2)
	m1.Lock()
	fmt.Println(m1 == m2)
	m1.Unlock()
	fmt.Println(m1 == m2)
}

// Output:
// a locked, b free: true true
// 7 8 9
// copy is locked: true
// copy now free: true
// original still locked: true
// true
// false
// true
