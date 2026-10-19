// phase: 07

// A generic priority queue: a type with a comparison function field, methods that call each other.
package main

import "fmt"

type Heap[T any] struct {
	data []T
	less func(a, b T) bool
}

func NewHeap[T any](less func(a, b T) bool) *Heap[T] { return &Heap[T]{less: less} }

func (h *Heap[T]) Len() int { return len(h.data) }

func (h *Heap[T]) Push(v T) {
	h.data = append(h.data, v)
	i := len(h.data) - 1
	for i > 0 {
		p := (i - 1) / 2
		if !h.less(h.data[i], h.data[p]) {
			break
		}
		h.data[i], h.data[p] = h.data[p], h.data[i]
		i = p
	}
}

func (h *Heap[T]) Pop() (T, bool) {
	var zero T
	if len(h.data) == 0 {
		return zero, false
	}
	top := h.data[0]
	last := len(h.data) - 1
	h.data[0] = h.data[last]
	h.data = h.data[:last]
	i := 0
	for {
		l, r, m := 2*i+1, 2*i+2, i
		if l < len(h.data) && h.less(h.data[l], h.data[m]) {
			m = l
		}
		if r < len(h.data) && h.less(h.data[r], h.data[m]) {
			m = r
		}
		if m == i {
			break
		}
		h.data[i], h.data[m] = h.data[m], h.data[i]
		i = m
	}
	return top, true
}

type Task struct {
	Name string
	Pri  int
}

func main() {
	h := NewHeap(func(a, b int) bool { return a < b })
	for _, v := range []int{5, 3, 8, 1, 9, 2} {
		h.Push(v)
	}
	var out []int
	for h.Len() > 0 {
		v, _ := h.Pop()
		out = append(out, v)
	}
	fmt.Println(out)
	t := NewHeap(func(a, b Task) bool { return a.Pri > b.Pri })
	t.Push(Task{"low", 1})
	t.Push(Task{"high", 9})
	t.Push(Task{"mid", 5})
	first, _ := t.Pop()
	second, _ := t.Pop()
	fmt.Println(first, second, t.Len())
	_, ok := NewHeap(func(a, b string) bool { return a < b }).Pop()
	fmt.Println(ok)
}

// Output:
// [1 2 3 5 8 9]
// {high 9} {mid 5} 1
// false
