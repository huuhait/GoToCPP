// phase: 08

// A Pool hands back what was Put, or what New makes; Put(nil) is ignored.
package main

import (
	"fmt"
	"sync"
)

type buf struct{ id int }

func main() {
	made := 0
	p := sync.Pool{New: func() any { made++; return &buf{id: made} }}
	a := p.Get().(*buf)
	b := p.Get().(*buf)
	fmt.Println("made:", made, a.id, b.id)
	p.Put(a)
	p.Put(b)
	p.Put(nil)
	seen := map[int]bool{}
	for i := 0; i < 2; i++ {
		x := p.Get().(*buf)
		seen[x.id] = true
	}
	fmt.Println("reused both:", seen[1] && seen[2], "made:", made)
	c := p.Get().(*buf)
	fmt.Println("new one:", c.id, "made:", made)

	var empty sync.Pool
	fmt.Println(empty.Get())
	empty.Put("x")
	fmt.Println(empty.Get(), empty.Get())

	// Items of different types.
	var mixed sync.Pool
	mixed.Put(1)
	mixed.Put("two")
	n := 0
	for mixed.Get() != nil {
		n++
	}
	fmt.Println("items:", n)
}

// Output:
// made: 2 1 2
// reused both: true made: 2
// new one: 3 made: 3
// <nil>
// x <nil>
// items: 2
