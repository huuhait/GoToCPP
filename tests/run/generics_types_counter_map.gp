// phase: 07

// A generic multiset: a map from keys to counts, methods that read, write and iterate.
package main

import "fmt"

type Counter[K comparable] struct{ m map[K]int }

func NewCounter[K comparable](keys ...K) *Counter[K] {
	c := &Counter[K]{m: map[K]int{}}
	for _, k := range keys {
		c.Inc(k)
	}
	return c
}

func (c *Counter[K]) Inc(k K) int {
	c.m[k]++
	return c.m[k]
}

func (c Counter[K]) Count(k K) int { return c.m[k] }

func (c Counter[K]) Total() int {
	t := 0
	for _, n := range c.m {
		t += n
	}
	return t
}

func (c Counter[K]) MostCommon() (K, int) {
	var best K
	n := -1
	for k, v := range c.m {
		if v > n {
			best, n = k, v
		}
	}
	return best, n
}

type Word string

func main() {
	c := NewCounter("a", "b", "a", "c", "a")
	fmt.Println(c.Count("a"), c.Count("z"), c.Total())
	fmt.Println(c.MostCommon())
	w := NewCounter[Word]()
	w.Inc("x")
	w.Inc("x")
	fmt.Println(w.MostCommon())
	p := NewCounter([2]int{1, 2}, [2]int{1, 2}, [2]int{3, 4})
	fmt.Println(p.Count([2]int{1, 2}), len(p.m))
}

// Output:
// 3 0 5
// a 3
// x 2
// 2 2
