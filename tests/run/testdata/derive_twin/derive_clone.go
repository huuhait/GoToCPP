// #Derive(Clone) (step 10.10): a deep copy. Slices, maps, arrays, pointers and structs are copied
// all the way down; nil stays nil and an empty slice or map stays empty; channels, funcs and
// interface values are shared (an interface that has a Clone method is cloned through it); a type
// of another package is copied as a value (time.Time), and a type with a Clone method of its own
// is cloned with it.

package main

import (
	"fmt"
	"time"
)

type Flat struct {
	A int
	B string
	C [2]int
}

type Leaf struct {
	Name string
	Vals []int
}

// Cloner is an interface with a Clone method: a field of this type is cloned through it.
type Cloner interface {
	Clone() Cloner
	ID() int
}

type Blob struct{ id *int }

func (b Blob) Clone() Cloner { n := *b.id + 1; return Blob{&n} }
func (b Blob) ID() int       { return *b.id }

type Counter struct{ N *int }

// Clone is written by hand: the derived Clone of Doc calls it.
func (c Counter) Clone() Counter {
	if c.N == nil {
		return c
	}
	n := *c.N + 1000
	return Counter{&n}
}

type Doc struct {
	Title    string
	Flat     Flat
	Leaves   []Leaf
	ByName   map[string]*Leaf
	Matrix   [2][]int
	Nested   map[string]map[string][]int
	Parent   *Doc
	Counter  Counter
	Counters []Counter
	When     time.Time
	Raw      []byte
	Anon     struct {
		Items []Leaf
		N     int
	}
	Opaque any
	Item   Cloner
	Done   chan struct{}
	Hook   func() int
	Ptrs   []*int
	Deep   **int
}

type Names []string

type Table map[string][]Leaf

type Cube [2][2][]int

type Chain struct {
	Val  int
	Next *Chain
}

func main() {
	seven := 7
	pseven := &seven
	d := Doc{
		Title:    "t",
		Flat:     Flat{1, "b", [2]int{3, 4}},
		Leaves:   []Leaf{{"l1", []int{1, 2}}, {"l2", nil}, {"l3", []int{}}},
		ByName:   map[string]*Leaf{"a": {"la", []int{9}}, "nil": nil},
		Matrix:   [2][]int{{1}, nil},
		Nested:   map[string]map[string][]int{"x": {"y": {5, 6}}, "e": {}},
		Parent:   &Doc{Title: "parent", Leaves: []Leaf{{"pl", []int{0}}}},
		Counter:  Counter{&seven},
		Counters: []Counter{{&seven}},
		When:     time.Unix(42, 0),
		Raw:      []byte("raw"),
		Opaque:   []int{1},
		Item:     Blob{&seven},
		Done:     make(chan struct{}),
		Hook:     func() int { return seven },
		Ptrs:     []*int{&seven, nil},
		Deep:     &pseven,
	}
	d.Anon.Items = []Leaf{{"an", []int{8}}}
	d.Anon.N = 2

	c := d.Clone()

	// the copy has the same values (printed through a helper, since Doc has no Equal)
	show := func(x Doc) string {
		p := "-"
		if x.Parent != nil {
			p = fmt.Sprint(x.Parent.Title, x.Parent.Leaves)
		}
		return fmt.Sprint(x.Title, x.Flat, x.Leaves, *x.ByName["a"], x.ByName["nil"], x.Matrix, x.Nested, p, *x.Counter.N, *x.Counters[0].N, x.When.Unix(), string(x.Raw), x.Anon, x.Opaque, x.Item.ID(), x.Hook(), *x.Ptrs[0], x.Ptrs[1], **x.Deep)
	}
	fmt.Println(show(d))
	fmt.Println(show(c))

	// nil stays nil, empty stays empty
	fmt.Println(c.Leaves[1].Vals == nil, c.Leaves[2].Vals == nil, len(c.Leaves[2].Vals), c.Matrix[1] == nil, c.ByName["nil"] == nil, c.Nested["e"] != nil)
	var zero Doc
	z := zero.Clone()
	fmt.Println(z.Leaves == nil, z.ByName == nil, z.Parent == nil, z.Raw == nil, z.Done == nil, z.Hook == nil, z.Deep == nil)

	// changing the copy leaves the original alone
	c.Title = "changed"
	c.Flat.C[0] = 99
	c.Leaves[0].Vals[0] = 99
	c.Leaves[0].Name = "changed"
	c.ByName["a"].Vals[0] = 99
	c.ByName["new"] = &Leaf{}
	c.Matrix[0][0] = 99
	c.Nested["x"]["y"][0] = 99
	c.Nested["x"]["z"] = nil
	c.Parent.Title = "changed"
	c.Parent.Leaves[0].Vals[0] = 99
	c.Raw[0] = 'X'
	c.Anon.Items[0].Vals[0] = 99
	*c.Ptrs[0] = 99
	*c.Deep = &seven
	fmt.Println(show(d))

	// what is shared: the channel, the func, the interface value; the hand-written Clone ran
	fmt.Println(c.Done == d.Done, c.Opaque.([]int)[0], *c.Counter.N, *c.Counters[0].N, seven, c.Item.ID(), d.Item.ID())

	// the copy of the copy is independent too
	cc := c.Clone()
	cc.Leaves[0].Vals[0] = -1
	fmt.Println(c.Leaves[0].Vals[0], cc.Leaves[0].Vals[0])

	// types that are not structs
	n := Names{"a", "b"}
	n2 := n.Clone()
	n2[0] = "z"
	fmt.Println(n, n2, Names(nil).Clone() == nil, Names{}.Clone() != nil)
	t := Table{"k": {{"a", []int{1}}}, "n": nil}
	t2 := t.Clone()
	t2["k"][0].Vals[0] = 5
	t2["k"][0].Name = "b"
	fmt.Println(t, t2, t2["n"] == nil)
	cube := Cube{{{1}, {2}}, {{3}, nil}}
	cube2 := cube.Clone()
	cube2[0][0][0] = 7
	fmt.Println(cube, cube2)
	ch := &Chain{1, &Chain{2, &Chain{3, nil}}}
	ch2 := ch.Clone()
	ch2.Next.Next.Val = 30
	ch2.Next.Next.Next = &Chain{4, nil}
	fmt.Println(ch.Val, ch.Next.Val, ch.Next.Next.Val, ch.Next.Next.Next == nil, ch2.Next.Next.Val, ch2.Next.Next.Next.Val)
	fmt.Println(ch.Next != ch2.Next, ch.Next.Next != ch2.Next.Next)
}

// The methods, written the way a Go programmer would.

func cloneInts(a []int) []int {
	if a == nil {
		return nil
	}
	return append(make([]int, 0, len(a)), a...)
}

func (l Leaf) Clone() Leaf { return Leaf{l.Name, cloneInts(l.Vals)} }

func cloneLeaves(a []Leaf) []Leaf {
	if a == nil {
		return nil
	}
	out := make([]Leaf, len(a))
	for i, l := range a {
		out[i] = l.Clone()
	}
	return out
}

func (d Doc) Clone() Doc {
	out := d // value fields, channel, func and interface value are shared or copied by =
	out.Leaves = cloneLeaves(d.Leaves)
	if d.ByName != nil {
		out.ByName = make(map[string]*Leaf, len(d.ByName))
		for k, v := range d.ByName {
			if v != nil {
				c := v.Clone()
				out.ByName[k] = &c
			} else {
				out.ByName[k] = nil
			}
		}
	}
	for i := range d.Matrix {
		out.Matrix[i] = cloneInts(d.Matrix[i])
	}
	if d.Nested != nil {
		out.Nested = make(map[string]map[string][]int, len(d.Nested))
		for k, m := range d.Nested {
			if m == nil {
				out.Nested[k] = nil
				continue
			}
			mm := make(map[string][]int, len(m))
			for k2, v := range m {
				mm[k2] = cloneInts(v)
			}
			out.Nested[k] = mm
		}
	}
	if d.Parent != nil {
		p := d.Parent.Clone()
		out.Parent = &p
	}
	out.Counter = d.Counter.Clone()
	if d.Counters != nil {
		out.Counters = make([]Counter, len(d.Counters))
		for i, c := range d.Counters {
			out.Counters[i] = c.Clone()
		}
	}
	if d.Raw != nil {
		out.Raw = append([]byte{}, d.Raw...)
	}
	out.Anon.Items = cloneLeaves(d.Anon.Items)
	if d.Item != nil {
		out.Item = d.Item.Clone()
	}
	if d.Ptrs != nil {
		out.Ptrs = make([]*int, len(d.Ptrs))
		for i, p := range d.Ptrs {
			if p != nil {
				v := *p
				out.Ptrs[i] = &v
			}
		}
	}
	if d.Deep != nil {
		inner := *d.Deep
		if inner != nil {
			v := *inner
			inner = &v
		}
		out.Deep = &inner
	}
	return out
}

func (n Names) Clone() Names {
	if n == nil {
		return nil
	}
	return append(Names{}, n...)
}

func (t Table) Clone() Table {
	if t == nil {
		return nil
	}
	out := make(Table, len(t))
	for k, v := range t {
		out[k] = cloneLeaves(v)
	}
	return out
}

func (c Cube) Clone() Cube {
	var out Cube
	for i := range c {
		for j := range c[i] {
			out[i][j] = cloneInts(c[i][j])
		}
	}
	return out
}

func (c Chain) Clone() Chain {
	out := c
	if c.Next != nil {
		n := c.Next.Clone()
		out.Next = &n
	}
	return out
}
