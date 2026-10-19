// phase: 08

// Instances of generic types as type arguments of other instances, and %T with nested names.
package main

import "fmt"

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

type Point struct{ X, Y int }

type Node struct{ next *Node }

func main() {
	a := Pair[string, Stack[int]]{"s", Stack[int]{[]int{1, 2}}}
	fmt.Printf("%T\n%v\n%+v\n", a, a, a)
	b := Pair[Point, []*Node]{Point{1, 2}, []*Node{nil}}
	fmt.Printf("%T %v\n", b, b)
	var c Stack[Pair[int, Stack[string]]]
	c.Push(Pair[int, Stack[string]]{1, Stack[string]{[]string{"x"}}})
	fmt.Printf("%T %v\n", c, c)
	d := map[string]Stack[Pair[bool, bool]]{"k": {}}
	fmt.Printf("%T %v\n", d, d)
	e := [2]Pair[int, func(int) int]{}
	fmt.Printf("%T %v\n", e, e[0].Val == nil)
	f := Pair[struct{ A int }, map[string][]int]{}
	fmt.Printf("%T\n", f)
	fmt.Printf("%T\n", Pair[Pair[int, int], Pair[string, string]]{})
	fmt.Printf("%T\n", &Pair[chan int, <-chan string]{})
}

// Output:
// main.Pair[string,main.Stack[int]]
// {s {[1 2]}}
// {Key:s Val:{items:[1 2]}}
// main.Pair[main.Point,[]*main.Node] {{1 2} [<nil>]}
// main.Stack[main.Pair[int,main.Stack[string]]] {[{1 {[x]}}]}
// map[string]main.Stack[main.Pair[bool,bool]] map[k:{[]}]
// [2]main.Pair[int,func(int) int] true
// main.Pair[struct { A int },map[string][]int]
// main.Pair[main.Pair[int,int],main.Pair[string,string]]
// *main.Pair[chan int,<-chan string]
