// phase: 08

// Pointers to structs travel through a three-stage pipeline: each stage owns the value in turn.
package main

import "fmt"

type Item struct {
	id   int
	text string
	tags []string
}

func main() {
	a := make(chan *Item)
	b := make(chan *Item)
	c := make(chan *Item)
	go func() {
		for i := 0; i < 20; i++ {
			a <- &Item{id: i}
		}
		close(a)
	}()
	go func() {
		for it := range a {
			it.text = fmt.Sprint("item", it.id)
			b <- it
		}
		close(b)
	}()
	go func() {
		for it := range b {
			it.tags = append(it.tags, it.text, "x")
			c <- it
		}
		close(c)
	}()
	n := 0
	for it := range c {
		n += len(it.tags) + it.id
	}
	fmt.Println(n)
}

// Output:
// 230
