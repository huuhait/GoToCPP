// bench: generic_stack
// category: generics
// size: 4000000
// smoke: 20000
// twin: no
// about: a generic Stack[T] type (push, pop, peek) instantiated for int, string and a struct
package main

import "fmt"

type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(x T) { s.items = append(s.items, x) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	x := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return x, true
}

func (s *Stack[T]) Len() int { return len(s.items) }

type point struct{ x, y int }

func main() {
	var is Stack[int]
	var ss Stack[string]
	var ps Stack[point]
	names := []string{"a", "bb", "ccc", "dddd"}
	sum := 0
	for i := 0; i < benchN; i++ {
		is.Push(i)
		ss.Push(names[i&3])
		ps.Push(point{i, -i})
		if i%3 == 2 {
			a, _ := is.Pop()
			b, _ := ss.Pop()
			c, _ := ps.Pop()
			sum += a + len(b) + c.x + c.y
		}
	}
	fmt.Println(sum, is.Len(), ss.Len(), ps.Len())
}
