// phase: 07
// lsan: cycle

// A generic cache with a map and a doubly linked list (prev and next are strong: a reference cycle that reference counting never frees, D1: lsan: cycle), eviction, and a constructor returning a pointer to an instance.
package main

import "fmt"

type entry[K comparable, V any] struct {
	key  K
	val  V
	prev *entry[K, V]
	next *entry[K, V]
}

type LRU[K comparable, V any] struct {
	cap        int
	items      map[K]*entry[K, V]
	head, tail *entry[K, V]
	evicted    []K
}

func NewLRU[K comparable, V any](cap int) *LRU[K, V] {
	return &LRU[K, V]{cap: cap, items: map[K]*entry[K, V]{}}
}

func (c *LRU[K, V]) unlink(e *entry[K, V]) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev, e.next = nil, nil
}

func (c *LRU[K, V]) front(e *entry[K, V]) {
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

func (c *LRU[K, V]) Put(k K, v V) {
	if e, ok := c.items[k]; ok {
		e.val = v
		c.unlink(e)
		c.front(e)
		return
	}
	if len(c.items) == c.cap {
		old := c.tail
		c.unlink(old)
		delete(c.items, old.key)
		c.evicted = append(c.evicted, old.key)
	}
	e := &entry[K, V]{key: k, val: v}
	c.items[k] = e
	c.front(e)
}

func (c *LRU[K, V]) Get(k K) (V, bool) {
	e, ok := c.items[k]
	if !ok {
		var z V
		return z, false
	}
	c.unlink(e)
	c.front(e)
	return e.val, true
}

func (c *LRU[K, V]) Keys() []K {
	var ks []K
	for e := c.head; e != nil; e = e.next {
		ks = append(ks, e.key)
	}
	return ks
}

func main() {
	c := NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Put("c", 3)
	fmt.Println(c.Keys(), c.evicted)
	fmt.Println(c.Get("b"))
	fmt.Println(c.Get("c"))
	d := NewLRU[int, []string](1)
	d.Put(1, []string{"x"})
	d.Put(2, nil)
	fmt.Println(d.Keys(), d.evicted)
	fmt.Printf("%T\n", d)
}

// Output:
// [c a] [b]
// 0 false
// 3 true
// [2] [1]
// *main.LRU[int,[]string]
