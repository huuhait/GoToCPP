// phase: 03

// An LRU cache: a map from keys to the nodes of a doubly linked list, most recent first. The
// prev pointers are weak, so the list has no reference cycle and evicted nodes are freed.
package main

import (
	"fmt"
	"weak"
)

type entry struct {
	key, val string
	next     *entry
	prev     weak.Pointer[entry]
}

type LRU struct {
	cap   int
	items map[string]*entry
	head  *entry
	tail  weak.Pointer[entry]
}

func NewLRU(n int) *LRU { return &LRU{cap: n, items: map[string]*entry{}} }

func (c *LRU) unlink(e *entry) {
	if p := e.prev.Value(); p != nil {
		p.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.next, e.prev = nil, weak.Pointer[entry]{}
}

func (c *LRU) pushFront(e *entry) {
	e.next = c.head
	if c.head != nil {
		c.head.prev = weak.Make(e)
	} else {
		c.tail = weak.Make(e)
	}
	c.head = e
}

func (c *LRU) Get(k string) (string, bool) {
	e, ok := c.items[k]
	if !ok {
		return "", false
	}
	c.unlink(e)
	c.pushFront(e)
	return e.val, true
}

func (c *LRU) Put(k, v string) (evicted string) {
	if e, ok := c.items[k]; ok {
		e.val = v
		c.unlink(e)
		c.pushFront(e)
		return ""
	}
	e := &entry{key: k, val: v}
	c.items[k] = e
	c.pushFront(e)
	if len(c.items) > c.cap {
		last := c.tail.Value()
		c.unlink(last)
		delete(c.items, last.key)
		return last.key
	}
	return ""
}

func (c *LRU) Keys() []string {
	var ks []string
	for e := c.head; e != nil; e = e.next {
		ks = append(ks, e.key)
	}
	return ks
}

func (c *LRU) KeysBack() []string {
	var ks []string
	for e := c.tail.Value(); e != nil; e = e.prev.Value() {
		ks = append(ks, e.key)
	}
	return ks
}

func main() {
	c := NewLRU(3)
	for _, kv := range [][2]string{{"a", "1"}, {"b", "2"}, {"c", "3"}} {
		c.Put(kv[0], kv[1])
	}
	fmt.Println(c.Keys(), c.KeysBack())
	v, ok := c.Get("a")
	fmt.Println(v, ok, c.Keys())
	fmt.Println("evicted:", c.Put("d", "4"), c.Keys(), c.KeysBack())
	fmt.Println("evicted:", c.Put("a", "10"), c.Keys())
	_, ok = c.Get("b")
	fmt.Println(ok, len(c.items))
	for i := 0; i < 10; i++ {
		c.Put(fmt.Sprint("k", i), fmt.Sprint(i))
	}
	fmt.Println(c.Keys(), c.KeysBack())
	v, _ = c.Get("k8")
	fmt.Println(v, c.Keys())
}

// Output:
// [c b a] [a b c]
// 1 true [a c b]
// evicted: b [d a c] [c a d]
// evicted:  [a d c]
// false 3
// [k9 k8 k7] [k7 k8 k9]
// 8 [k8 k9 k7]
