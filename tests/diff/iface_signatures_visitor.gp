// phase: 05

// Interface methods with multiple results, func parameters, variadics, interface results (named and unnamed), local interface types.
package main

import "fmt"

type Visitor interface {
	Visit(n Node) (w Visitor)
}
type Node interface {
	Children() []Node
	Accept(Visitor) Visitor
	fmt.Stringer
}
type Leaf struct{ v int }
type Branch struct {
	name string
	kids []Node
}

func (l Leaf) Children() []Node         { return nil }
func (l Leaf) Accept(v Visitor) Visitor { return v.Visit(l) }
func (l Leaf) String() string           { return fmt.Sprint(l.v) }
func (b *Branch) Children() []Node      { return b.kids }
func (b *Branch) Accept(v Visitor) Visitor {
	w := v.Visit(b)
	if w == nil {
		return nil
	}
	for _, k := range b.kids {
		k.Accept(w)
	}
	return w
}
func (b *Branch) String() string { return b.name }

type depthPrinter struct{ depth int }

func (d depthPrinter) Visit(n Node) Visitor {
	fmt.Printf("%*s%v\n", d.depth*2, "", n)
	if n.String() == "skip" {
		return nil
	}
	return depthPrinter{d.depth + 1}
}

type Mapper interface {
	Map(func(int) int) Mapper
	Reduce(init int, f func(acc, x int) int) int
	Pair() (int, Mapper)
	Variadic(prefix string, xs ...int) string
	Self() interface{ Self() Mapper }
}
type ints []int

func (s ints) Map(f func(int) int) Mapper {
	out := make(ints, len(s))
	for i, x := range s {
		out[i] = f(x)
	}
	return out
}
func (s ints) Reduce(acc int, f func(acc, x int) int) int {
	for _, x := range s {
		acc = f(acc, x)
	}
	return acc
}
func (s ints) Pair() (int, Mapper) { return len(s), s }
func (s ints) Variadic(p string, xs ...int) string {
	return fmt.Sprint(p, len(xs), xs, len(s))
}
func (s ints) Self() interface{ Self() Mapper } { return selfer{s} }

type selfer struct{ m Mapper }

func (s selfer) Self() Mapper { return s.m }

func main() {
	tree := &Branch{"root", []Node{Leaf{1}, &Branch{"skip", []Node{Leaf{99}}}, &Branch{"b", []Node{Leaf{2}, Leaf{3}}}}}
	tree.Accept(depthPrinter{})
	var m Mapper = ints{1, 2, 3}
	n, m2 := m.Pair()
	m3 := m2.Map(func(x int) int { return x * x }).Map(func(x int) int { return x + 1 })
	fmt.Println(n, m3, m3.Reduce(0, func(a, x int) int { return a + x }))
	fmt.Println(m.Variadic("a"), m.Variadic("b", 1, 2), m.Variadic("c", []int{4, 5, 6}...))
	fmt.Println(m.Self().Self())
	reduce := m.Reduce
	fmt.Println(reduce(100, func(a, x int) int { return a - x }))
	mexpr := Mapper.Variadic
	fmt.Println(mexpr(m, "d", 7))
	type local interface{ Reduce(int, func(int, int) int) int }
	var l local = m
	fmt.Println(l.Reduce(1, func(a, x int) int { return a * (x + 1) }))
	_, isNode := any(m).(Node)
	_, isLocal := any(ints{}).(local)
	fmt.Println(isNode, isLocal)
}
