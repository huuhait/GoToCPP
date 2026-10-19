// phase: 09
// lsan: cycle

// Recursive and mutually recursive generic types: a tree whose nodes hold a forest, a forest of
// trees, a linked list of pairs of itself, an expression type parameterized by its leaf, and methods
// that recurse across the instances; the %T names stay finite.
package main

import (
	"fmt"
	"strings"
)

type Tree[T any] struct {
	Val    T
	Kids   Forest[T]
	Parent *Tree[T]
}

type Forest[T any] []*Tree[T]

func (t *Tree[T]) Add(v T) *Tree[T] {
	k := &Tree[T]{Val: v, Parent: t}
	t.Kids = append(t.Kids, k)
	return k
}

func (f Forest[T]) Walk(depth int, visit func(int, T)) {
	for _, t := range f {
		t.Walk(depth, visit)
	}
}

func (t *Tree[T]) Walk(depth int, visit func(int, T)) {
	visit(depth, t.Val)
	t.Kids.Walk(depth+1, visit)
}

func (t *Tree[T]) Depth() int {
	d := 0
	for p := t.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

type Expr[L any] interface {
	Eval(env map[string]L) L
	Show() string
}

type Lit[L any] struct{ V L }
type Var[L any] struct{ Name string }
type Bin[L any] struct {
	Op   string
	A, B Expr[L]
	Fn   func(a, b L) L
}

func (l Lit[L]) Eval(map[string]L) L   { return l.V }
func (l Lit[L]) Show() string          { return fmt.Sprint(l.V) }
func (v Var[L]) Eval(e map[string]L) L { return e[v.Name] }
func (v Var[L]) Show() string          { return v.Name }
func (b Bin[L]) Eval(e map[string]L) L { return b.Fn(b.A.Eval(e), b.B.Eval(e)) }
func (b Bin[L]) Show() string          { return "(" + b.A.Show() + b.Op + b.B.Show() + ")" }

type Chain[A, B any] struct {
	Head A
	Rest *Chain[B, A]
}

func (c *Chain[A, B]) Len() int {
	if c == nil {
		return 0
	}
	return 1 + c.Rest.Len()
}

func main() {
	root := &Tree[string]{Val: "root"}
	a := root.Add("a")
	a.Add("a1").Add("a1x")
	a.Add("a2")
	root.Add("b")
	root.Kids.Walk(1, func(d int, s string) { fmt.Println(strings.Repeat("  ", d), s) })
	leaf := a.Kids[0].Kids[0]
	fmt.Println(leaf.Depth(), leaf.Parent.Val, root.Depth())
	fmt.Printf("%T %T %T\n", root, root.Kids, root.Kids[0].Kids)

	add := func(a, b int8) int8 { return a + b }
	mul := func(a, b int8) int8 { return a * b }
	e := Bin[int8]{"+", Bin[int8]{"*", Var[int8]{"x"}, Lit[int8]{20}, mul}, Lit[int8]{100}, add}
	fmt.Println(e.Show(), e.Eval(map[string]int8{"x": 7}), e.Eval(map[string]int8{"x": 2}))
	fmt.Printf("%T %T\n", e, e.A)
	fe := Bin[float64]{"/", Lit[float64]{1}, Var[float64]{"z"}, func(a, b float64) float64 { return a / b }}
	fmt.Println(fe.Show(), fe.Eval(map[string]float64{"z": 0}), fe.Eval(map[string]float64{"z": 8}))

	c := &Chain[int, string]{1, &Chain[string, int]{"two", &Chain[int, string]{3, nil}}}
	fmt.Println(c.Len(), c.Head, c.Rest.Head, c.Rest.Rest.Head)
	fmt.Printf("%T %T\n", c, c.Rest)
}

// Output:
//    a
//      a1
//        a1x
//      a2
//    b
// 3 a1 0
// *main.Tree[string] main.Forest[string] main.Forest[string]
// ((x*20)+100) -16 -116
// main.Bin[int8] main.Bin[int8]
// (1/z) +Inf 0.125
// 3 1 two 3
// *main.Chain[int,string] *main.Chain[string,int]
