// phase: 08

// A Cond that is copied after first use panics, as Go's copyChecker does; a copy made before is fine.
package main

import (
	"fmt"
	"sync"
)

func try(name string, f func()) {
	defer func() { fmt.Println(name+":", recover()) }()
	f()
}

func main() {
	var mu sync.Mutex
	c1 := sync.NewCond(&mu)
	c1.Signal()
	c2 := *c1
	try("signal on a copy", func() { c2.Signal() })
	try("broadcast on a copy", func() { c2.Broadcast() })
	try("wait on a copy", func() {
		mu.Lock()
		defer mu.Unlock()
		c2.Wait()
	})
	try("original", func() { c1.Broadcast() })

	fresh := sync.Cond{L: &mu}
	copy1 := fresh
	try("copy before use", func() { copy1.Signal() })
}

// Output:
// signal on a copy: sync.Cond is copied
// broadcast on a copy: sync.Cond is copied
// wait on a copy: sync.Cond is copied
// original: <nil>
// copy before use: <nil>
