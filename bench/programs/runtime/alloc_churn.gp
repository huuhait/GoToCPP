// bench: alloc_churn
// category: runtime
// size: 3000000
// smoke: 5000
// twin: yes
// about: allocate and drop small structs, slices and strings in a loop (allocator and reference counting)
package main

import "fmt"

type item struct {
	id    int
	name  string
	attrs []int
	next  *item
}

func main() {
	var keep [64]*item
	sum := 0
	seed := uint64(5)
	for i := 0; i < benchN; i++ {
		seed = seed*6364136223846793005 + 1442695040888963407
		it := &item{id: i, attrs: make([]int, int(seed>>60)+1)}
		it.attrs[0] = i
		if i%7 == 0 {
			it.name = "item-" + string(rune('a'+i%26))
		}
		slot := int(seed>>33) & 63
		it.next = keep[slot]
		keep[slot] = it
		if i&15 == 0 {
			keep[slot] = nil
		}
		sum += len(it.attrs) + len(it.name) + it.attrs[0]%3
	}
	live := 0
	for _, p := range keep {
		for ; p != nil; p = p.next {
			live++
		}
	}
	fmt.Println(sum, live)
}
