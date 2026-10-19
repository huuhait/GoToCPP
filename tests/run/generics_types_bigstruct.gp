// phase: 07

// Large instances of generic types: value receivers, copies and by-value parameters bigger than a stack frame.
package main

import "fmt"

type Big[T any] struct {
	data [30000]T
	n    int
}

func (b *Big[T]) Set(i int, v T) { b.data[i] = v; b.n++ }
func (b Big[T]) At(i int) T      { return b.data[i] }
func (b Big[T]) Len() int        { return len(b.data) }

func Copy[T any](b Big[T]) Big[T] {
	b.Set(0, b.At(1))
	return b
}

type Holder[T any] struct{ inner Big[T] }

func main() {
	var b Big[int]
	b.Set(29999, 5)
	b.Set(1, 7)
	c := Copy(b)
	fmt.Println(b.At(0), c.At(0), b.At(29999), c.At(29999), b.n, c.n, b.Len())
	var h Holder[string]
	h.inner.Set(3, "x")
	h2 := h
	h2.inner.Set(3, "y")
	fmt.Println(h.inner.At(3), h2.inner.At(3))
	bs := []Big[bool]{{}, {}}
	bs[1].Set(0, true)
	fmt.Println(bs[0].At(0), bs[1].At(0))
}

// Output:
// 0 7 5 5 2 3 30000
// x y
// false true
