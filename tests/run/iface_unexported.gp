// phase: 05

// Interfaces with unexported methods: they match methods of the same package only (step 5.4).
package main

import "fmt"

type counter interface {
	inc() int
	Value() int
}

type c1 struct{ n int }
type c2 struct{ n int }

func (c *c1) inc() int  { c.n++; return c.n }
func (c *c1) Value() int { return c.n }
func (c c2) Value() int  { return c.n }

func main() {
	var c counter = &c1{}
	c.inc()
	fmt.Println(c.inc(), c.Value())
	var a any = c2{5}
	_, ok := a.(counter)
	fmt.Println(ok)
	_, ok = any(&c1{}).(counter)
	fmt.Println(ok)
	var v interface{ Value() int } = c2{3}
	fmt.Println(v.Value())
	_, ok = v.(counter)
	fmt.Println(ok)
}

// Output:
// 2 2
// false
// true
// 3
// false
