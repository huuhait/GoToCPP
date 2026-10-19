// phase: 07

// A recursive generic type: a linked list whose nodes point to nodes of the same instance.
package main

import "fmt"

type Node[T any] struct {
	Val  T
	Next *Node[T]
}

type List[T any] struct {
	head *Node[T]
	size int
}

func (l *List[T]) Push(v T) {
	l.head = &Node[T]{v, l.head}
	l.size++
}

func (l *List[T]) Each(f func(T)) {
	for n := l.head; n != nil; n = n.Next {
		f(n.Val)
	}
}

func (l *List[T]) Reverse() *List[T] {
	out := &List[T]{}
	l.Each(out.Push)
	return out
}

func (l *List[T]) Map(f func(T) T) *List[T] {
	out := &List[T]{}
	l.Each(func(v T) { out.Push(f(v)) })
	return out.Reverse()
}

func (l *List[T]) Slice() []T {
	var s []T
	l.Each(func(v T) { s = append(s, v) })
	return s
}

func main() {
	l := &List[int]{}
	for i := 1; i <= 4; i++ {
		l.Push(i)
	}
	fmt.Println(l.Slice(), l.Reverse().Slice(), l.size)
	fmt.Println(l.Map(func(i int) int { return i * 10 }).Slice())
	s := &List[string]{}
	s.Push("x")
	s.Push("y")
	fmt.Println(s.Slice(), s.head.Val, s.head.Next.Val, s.head.Next.Next == nil)
	fmt.Printf("%T %T\n", s.head, s)
}

// Output:
// [4 3 2 1] [1 2 3 4] 4
// [40 30 20 10]
// [y x] y x true
// *main.Node[string] *main.List[string]
