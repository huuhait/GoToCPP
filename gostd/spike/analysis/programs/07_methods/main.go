// Methods: value and pointer receivers, a method value (the receiver is copied when the value
// is made), method expressions, and a method promoted from an embedded type.
package main

type Counter struct{ n int }

func (c Counter) Get() int      { return c.n }
func (c *Counter) Inc()         { c.n++ }
func (c Counter) Add(d int) int { return c.n + d }

type Named struct{ Counter }

func main() {
	c := Counter{}
	c.Inc()
	c.Inc()
	println(c.Get(), c.Add(10))
	f := c.Get          // method value, receiver copied now
	g := (*Counter).Inc // method expression
	g(&c)
	println(c.Get(), f())
	h := Counter.Get
	println(h(c))
	var m Counter = c
	m.Inc()
	println(c.Get(), m.Get())
	n := Named{Counter{5}}
	n.Inc()
	println(n.Get(), n.n)
	println(n.Counter.Get())
}
