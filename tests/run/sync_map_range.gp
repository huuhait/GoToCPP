// phase: 08

// Range may call any method of the map: Delete during Range removes the entry (it is not shown if
// it was not visited yet), Store of existing keys is fine, and returning false stops the loop.
package main

import (
	"fmt"
	"sync"
)

func main() {
	var m sync.Map
	for i := 0; i < 10; i++ {
		m.Store(i, i*i)
	}
	visited := 0
	m.Range(func(k, v any) bool {
		visited++
		m.Delete(k) // delete what we see: each key is seen at most once
		return true
	})
	fmt.Println("visited:", visited)
	n := 0
	m.Range(func(k, v any) bool { n++; return true })
	fmt.Println("left:", n)

	for i := 0; i < 10; i++ {
		m.Store(i, 0)
	}
	stops := 0
	m.Range(func(k, v any) bool {
		stops++
		m.Store(k, 1)
		return stops < 3
	})
	fmt.Println("stopped after:", stops)
	ones := 0
	m.Range(func(k, v any) bool {
		if v == 1 {
			ones++
		}
		return true
	})
	fmt.Println("ones:", ones)
}

// Output:
// visited: 10
// left: 0
// stopped after: 3
// ones: 3
