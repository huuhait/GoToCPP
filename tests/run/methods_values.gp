// phase: 03

// Method values and method expressions (step 3.10b): a method value keeps a copy of a value
// receiver from when it was made, and the pointer of a pointer receiver; a method expression
// takes the receiver as its first parameter. Both are func values: stored, passed, kept in a
// map and called through a defer.
package main

import "fmt"

type Counter struct {
	name string
	n    int
}

func (c Counter) Show() int   { return c.n }
func (c *Counter) Inc(by int) { c.n += by }

type Op func(int, int) int

func (o Op) Apply(a, b int) int { return o(a, b) }

func twice(f func(int)) {
	f(1)
	f(1)
}

func main() {
	c := Counter{name: "c"}
	show := c.Show // a copy of c now
	inc := c.Inc   // &c
	twice(inc)
	inc(10)
	fmt.Println(show(), c.Show())

	byName := map[string]func() int{"now": c.Show, "then": show}
	fmt.Println(byName["now"](), byName["then"]())

	shower := Counter.Show
	incr := (*Counter).Inc
	incr(&c, 100)
	fmt.Println(shower(c), (*Counter).Show(&c))

	var add Op = func(a, b int) int { return a + b }
	apply := add.Apply
	fmt.Println(apply(2, 3), Op.Apply(add, 4, 5))

	defer fmt.Println("deferred:", show())
	final := c.Show
	c.n = 0
	defer fmt.Println(final())
}

// Output:
// 0 12
// 12 0
// 112 112
// 5 9
// 112
// deferred: 0
