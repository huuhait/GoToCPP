// phase: 03

// Pointers to structs and into them: &T{...}, new(T), fields through a pointer (auto-deref), &x.f
// of a local (which is then boxed), of a package variable, through a pointer and of a slice
// element, all sharing the owner of the struct they point into, so a pointer to a field keeps
// the whole struct alive after its function returns.
package main

import "fmt"

type Counter struct {
	name string
	n    int
	hist []int
}

var global Counter

func newCounter(name string) *Counter {
	return &Counter{name: name}
}

func bump(c *Counter, by int) {
	c.n += by
	c.hist = append(c.hist, by)
}

// field returns a pointer to a field of a local struct: it outlives the call.
func field() *int {
	c := Counter{n: 41}
	p := &c.n
	*p++
	return p
}

func counts() []Counter {
	return []Counter{{name: "a", n: 1}, {name: "b", n: 2}}
}

func main() {
	c := newCounter("c")
	bump(c, 2)
	bump(c, 3)
	fmt.Println(c.name, c.n, len(c.hist), c.hist[1])
	d := c
	d.n = 100
	fmt.Println(c.n, c == d)
	e := new(Counter)
	e.name = "e"
	fmt.Println(e == c, e.name, e.n, e.hist == nil)

	pn := &global.n
	*pn = 7
	bump(&global, 1)
	fmt.Println(global.n, *pn)

	p := field()
	fmt.Println(*p)

	cs := counts()
	pc := &cs[1].n
	*pc *= 10
	cs = nil
	fmt.Println(*pc)

	pf := &newCounter("f").n
	*pf = 5
	fmt.Println(*pf)
}

// Output:
// c 5 2 3
// 100 true
// false e 0 true
// 8 8
// 42
// 20
// 5
