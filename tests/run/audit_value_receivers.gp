// phase: 03

// Audit case: a value receiver is its own copy (Phase 03 README §2.5). Writes made through
// other pointers while the method runs, and a receiver whose object is freed meanwhile, don't
// change what the method sees; its own writes don't reach the caller's value.
package main

import "fmt"

type Box struct {
	vals [3]int
	name string
	tags []string
}

var cur *Box

func (b Box) sumAfter(f func()) int {
	f() // changes *cur, which is where b came from
	return b.vals[0] + b.vals[1] + b.vals[2]
}

func (b Box) rename(n string) Box {
	b.name = n
	b.vals[0] = 100
	return b
}

func (b Box) nameAfterFree() string {
	cur = nil // the only pointer to the object b was copied from
	return b.name
}

func (b *Box) bump() { b.vals[1]++ }

type Counter int

func (c Counter) twice(inc func()) (Counter, Counter) {
	before := c
	inc()
	return before, c
}

func main() {
	cur = &Box{vals: [3]int{1, 2, 3}, name: "a", tags: []string{"x"}}
	fmt.Println(cur.sumAfter(func() { cur.vals = [3]int{10, 20, 30} }), cur.vals)

	b := *cur
	r := b.rename("b")
	fmt.Println(b.name, b.vals, r.name, r.vals)

	// The slice header is copied, its array is shared (as in Go).
	t := cur.rename("t")
	t.tags[0] = "shared"
	fmt.Println(cur.tags, t.tags)

	fmt.Println(cur.nameAfterFree(), cur == nil)

	x := Box{}
	x.bump()
	x.bump()
	p := &x
	fmt.Println(p.sumAfter(func() { p.bump() }), x.vals)

	var c Counter = 5
	fmt.Println(c.twice(func() { c++ }))
	fmt.Println(c)
}

// Output:
// 6 [10 20 30]
// a [10 20 30] b [100 20 30]
// [shared] [shared]
// a true
// 2 [0 3 0]
// 5 5
// 6
