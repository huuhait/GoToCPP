// phase: 09

// Generic interfaces implemented by several generic types, held in slices and maps of the interface,
// used through type assertions, type switches and generic functions that take the interface
// instantiated with other type arguments; method sets of pointer and value receivers.
package main

import (
	"fmt"
	"strings"
)

type Container[T any] interface {
	Put(T)
	Get() (T, bool)
	Len() int
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Put(v T) { s.items = append(s.items, v) }
func (s *Stack[T]) Get() (v T, ok bool) {
	if len(s.items) == 0 {
		return
	}
	v, s.items = s.items[len(s.items)-1], s.items[:len(s.items)-1]
	return v, true
}
func (s *Stack[T]) Len() int { return len(s.items) }

type Queue[T any] struct{ items []T }

func (q *Queue[T]) Put(v T) { q.items = append(q.items, v) }
func (q *Queue[T]) Get() (v T, ok bool) {
	if len(q.items) == 0 {
		return
	}
	v, q.items = q.items[0], q.items[1:]
	return v, true
}
func (q *Queue[T]) Len() int { return len(q.items) }

type Ring[T any] struct {
	buf  [3]T
	n, i int
}

func (r *Ring[T]) Put(v T) { r.buf[(r.i+r.n)%3] = v; r.n = min(r.n+1, 3) }
func (r *Ring[T]) Get() (v T, ok bool) {
	if r.n == 0 {
		return
	}
	v = r.buf[r.i]
	r.i = (r.i + 1) % 3
	r.n--
	return v, true
}
func (r *Ring[T]) Len() int { return r.n }

type Counter[T comparable] struct{ m map[T]int }

func (c Counter[T]) Put(v T)        { c.m[v]++ }
func (c Counter[T]) Get() (T, bool) { var z T; return z, false }
func (c Counter[T]) Len() int       { return len(c.m) }
func (c Counter[T]) String() string { return fmt.Sprint("Counter", len(c.m)) }

func Fill[T any](c Container[T], vs ...T) Container[T] {
	for _, v := range vs {
		c.Put(v)
	}
	return c
}

func Drain[T any](c Container[T]) []T {
	var out []T
	for c.Len() > 0 {
		v, ok := c.Get()
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out
}

func kinds[T any](cs ...Container[T]) string {
	var parts []string
	for _, c := range cs {
		switch c := c.(type) {
		case *Stack[T]:
			parts = append(parts, "stack")
		case *Queue[T]:
			parts = append(parts, "queue")
		case *Ring[T]:
			parts = append(parts, fmt.Sprint("ring", c.n))
		case fmt.Stringer:
			parts = append(parts, c.String())
		default:
			parts = append(parts, fmt.Sprintf("%T", c))
		}
	}
	return strings.Join(parts, ",")
}

func main() {
	ints := []Container[int]{&Stack[int]{}, &Queue[int]{}, &Ring[int]{}, Counter[int]{map[int]int{}}}
	for _, c := range ints {
		Fill(c, 1, 2, 3, 4, 2)
		fmt.Printf("%T len=%d\n", c, c.Len())
	}
	fmt.Println(kinds(ints...))
	for _, c := range ints[:3] {
		fmt.Println(Drain(c))
	}
	strs := map[string]Container[string]{"s": &Stack[string]{}, "q": &Queue[string]{}, "r": &Ring[string]{}}
	for _, k := range []string{"q", "r", "s"} {
		fmt.Println(k, Drain(Fill(strs[k], "a", "b", "c", "d")))
	}
	var c Container[float64] = &Ring[float64]{}
	_, isStack := c.(*Stack[float64])
	_, isRing := c.(*Ring[float64])
	_, isOther := c.(Container[int])
	fmt.Println(isStack, isRing, isOther)
	cc, ok := any(c).(interface{ Len() int })
	fmt.Println(cc.Len(), ok)
	var nilc Container[int]
	fmt.Println(nilc == nil, kinds[int](nil))
}

// Output:
// *main.Stack[int] len=5
// *main.Queue[int] len=5
// *main.Ring[int] len=3
// main.Counter[int] len=4
// stack,queue,ring3,Counter4
// [2 4 3 2 1]
// [1 2 3 4 2]
// [2 2 3]
// q [a b c d]
// r [d b c]
// s [d c b a]
// false true false
// 0 true
// true <nil>
