// phase: 05

// Methods promoted through several levels of embedding, by value and by pointer, are called with a
// raw pointer to the embedded field (ADR-086): the pointer receiver of the innermost method
// is the address of a field inside the outer object, or an embedded pointer's object. The
// method overridden at an outer level, shadowing and interface calls go through the same path.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc()         { c.n++ }
func (c *Counter) Add(k int)    { c.n += k }
func (c Counter) Get() int      { return c.n }
func (c *Counter) Self() *Counter { return c } // returns its receiver: an owning pointer

type Named struct {
	Counter
	name string
}

func (n *Named) Add(k int) { n.Counter.Add(k * 2) } // overrides the promoted method

type Deep struct {
	*Named
	extra int
}

type Top struct {
	Deep
	Other *Counter
}

type Incer interface {
	Inc()
	Add(int)
	Get() int
}

type Self interface{ Self() *Counter }

func drive(i Incer, k int) int {
	i.Inc()
	i.Add(k)
	return i.Get()
}

// A function with a pointer parameter that only reads or writes through it, called with the
// address of a nested field.
func bump(c *Counter) { c.n += 100 }

func show(c *Counter) int { return c.n }

// A promoted pointer method of an embedded value, called through a borrowed receiver with an
// argument that runs code: the receiver's nil check comes first, in Go's order.
type Wrap struct {
	Counter
	tag string
}

var sideCalls int

func side(n int) int {
	sideCalls++
	return n
}

func (w *Wrap) bump() int {
	w.Add(side(2))
	w.Inc()
	return w.n + sideCalls
}

func sumPtr(x *int, k int) int { return *x + k }
func two(x, y *int, k int) int  { return *x + *y + k }

// The addresses of parts of a borrowed receiver go straight to borrowed parameters, next to
// arguments that run code.
func (w *Wrap) addrArgs() int {
	a := sumPtr(&w.n, side(3))
	b := two(&w.Counter.n, &w.Counter.n, side(1))
	c := sumPtr(&w.n, side(2))
	return a + b + c
}

func (t *Top) viaOther() int {
	t.Other.Inc()
	bump(t.Other)
	return t.Other.Get()
}

func main() {
	t := Top{Deep: Deep{Named: &Named{name: "x"}}, Other: &Counter{}}
	t.Inc()
	t.Add(3)
	t.Deep.Inc()
	t.Named.Counter.Add(1)
	fmt.Println(t.Get(), t.n, t.Counter.Get(), t.Named.n)
	bump(&t.Counter)
	bump(&t.Named.Counter)
	fmt.Println(show(&t.Counter), t.n)
	fmt.Println(t.viaOther())
	fmt.Println(drive(&t, 5), drive(t.Named, 1), drive(&t.Named.Counter, 2), drive(&t.Deep, 3))

	var s Self = &t
	fmt.Println(s.Self().n, t.Self() == &t.Named.Counter)

	// A copy shares the embedded pointer, not the embedded value.
	u := t
	u.Inc()
	fmt.Println(t.n, u.n, t.Other == u.Other)

	// Embedded by value in a boxed variable and in an array element.
	var arr [2]Named
	arr[1].Inc()
	arr[1].Add(2)
	bump(&arr[1].Counter)
	fmt.Println(arr[0].Get(), arr[1].Get())
	list := []*Named{{}, {}}
	list[1].Inc()
	list[1].Counter.Add(4)
	fmt.Println(list[0].n, list[1].n, drive(list[1], 7))
	m := map[string]*Deep{"a": {Named: &Named{}}}
	m["a"].Inc()
	m["a"].Add(1)
	fmt.Println(m["a"].n)

	w := &Wrap{}
	fmt.Println(w.bump(), w.bump(), sideCalls)
	fmt.Println(w.addrArgs(), sideCalls)
	var nw *Wrap
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil, sideCalls) }()
		nw.bump()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil, sideCalls) }()
		nw.addrArgs()
	}()

	f := t.Inc
	g := (*Top).Add
	f()
	g(&t, 1)
	fmt.Println(t.n)
}

// Output:
// 9 9 9 9
// 209 209
// 101
// 220 223 226 233
// 233 true
// 234 234 true
// 0 105
// 0 20 20
// 3
// 4 8 2
// 30 5
// recovered: true 6
// recovered: true 7
// 237
