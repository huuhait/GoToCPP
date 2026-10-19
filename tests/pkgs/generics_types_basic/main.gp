// phase: 07

// Generic types of another package, instantiated by main with predeclared types and its own types: the instance is one C++ type, written in main's header.
package main

import (
	"fmt"

	"example.com/gtbasic/coll"
)

type Point struct{ X, Y int }

type Celsius float64

func main() {
	var s coll.Stack[Point]
	s.Push(Point{1, 2})
	s.Push(Point{3, 4})
	p, ok := s.Pop()
	fmt.Println(p, ok, s.Len())
	st := coll.New("a", "b", "c")
	fmt.Println(st.Len(), *st)
	pair := coll.Pair[string, Celsius]{"t", 36.6}
	fmt.Println(pair, pair.Key, pair.Val)
	fmt.Printf("%T %T %T\n", s, st, pair)
	set := coll.Set[Point]{}
	set.Add(Point{1, 1})
	fmt.Println(set.Has(Point{1, 1}), set.Has(Point{2, 2}), len(set))
	var any1 any = st
	_, isStr := any1.(*coll.Stack[string])
	_, isInt := any1.(*coll.Stack[int])
	fmt.Println(isStr, isInt)
	nested := coll.Pair[coll.Pair[int, int], *coll.Stack[Celsius]]{}
	fmt.Printf("%T %v\n", nested, nested.Val == nil)
}

// Output:
// {3 4} true 1
// 3 {[a b c]}
// t:36.6 t 36.6
// coll.Stack[main.Point] *coll.Stack[string] coll.Pair[string,main.Celsius]
// true false 1
// true false
// coll.Pair[example.com/gtbasic/coll.Pair[int,int],*example.com/gtbasic/coll.Stack[main.Celsius]] true
