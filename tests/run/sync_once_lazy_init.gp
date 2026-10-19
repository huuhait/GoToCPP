// phase: 08

// The lazy-initialisation pattern: a Once in a struct, one instance per struct, copied before use.
package main

import (
	"fmt"
	"sync"
)

type Lazy struct {
	once sync.Once
	val  int
	n    *int
}

func (l *Lazy) Get() int {
	l.once.Do(func() {
		*l.n++
		l.val = 100 + *l.n
	})
	return l.val
}

func main() {
	count := 0
	proto := Lazy{n: &count}
	a, b := proto, proto
	var wg sync.WaitGroup
	got := make([]int, 40)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				got[i] = a.Get()
			} else {
				got[i] = b.Get()
			}
		}()
	}
	wg.Wait()
	fmt.Println("initialisations:", count)
	fmt.Println(a.Get()+b.Get() == 201+202 || a.Get()+b.Get() == 202+201)
	seenA, seenB := got[0], got[1]
	same := true
	for i := range got {
		if i%2 == 0 && got[i] != seenA || i%2 == 1 && got[i] != seenB {
			same = false
		}
	}
	fmt.Println("each copy has one value:", same, seenA != seenB)
}

// Output:
// initialisations: 2
// false
// each copy has one value: true true
