// phase: 07

// A generic stack with pointer and value methods, used with several element types.
package main

import "fmt"

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}

func (s Stack[T]) Len() int { return len(s.items) }

func (s Stack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

type Point struct{ X, Y int }

func main() {
	var ints Stack[int]
	for i := 1; i <= 3; i++ {
		ints.Push(i * i)
	}
	fmt.Println(ints.Len())
	fmt.Println(ints.Peek())
	var popped []int
	for {
		v, ok := ints.Pop()
		if !ok {
			break
		}
		popped = append(popped, v)
	}
	fmt.Println(popped)
	strs := &Stack[string]{}
	strs.Push("a")
	strs.Push("b")
	fmt.Println(strs.Pop())
	fmt.Println(strs.Len(), *strs)
	pts := Stack[Point]{}
	pts.Push(Point{1, 2})
	fmt.Println(pts.Peek())
	fmt.Println(Stack[[]int]{}.Len(), Stack[func()]{}.Len())
	fmt.Printf("%T %T %T\n", ints, strs, pts)
}

// Output:
// 3
// 9 true
// [9 4 1]
// b true
// 1 {[a]}
// {1 2} true
// 0 0
// main.Stack[int] *main.Stack[string] main.Stack[main.Point]
