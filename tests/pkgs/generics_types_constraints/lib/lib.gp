package lib

import "fmt"

type Stringer interface{ String() string }

type Number interface{ ~int | ~int8 | ~float64 }

// Labeled calls a method of its type argument, which a type of another package has.
type Labeled[T Stringer] struct {
	items []T
	sep   string
}

func NewLabeled[T Stringer](sep string, items ...T) Labeled[T] { return Labeled[T]{items, sep} }

func (l Labeled[T]) Join() string {
	out := ""
	for i, it := range l.items {
		if i > 0 {
			out += l.sep
		}
		out += it.String()
	}
	return out
}

type Acc[T Number] struct{ total T }

func (a *Acc[T]) Add(v T) *Acc[T] { a.total += v; return a }
func (a Acc[T]) Total() T         { return a.total }
func (a Acc[T]) Average(n int) T  { return a.total / T(n) }

type Index[K comparable, V fmt.Stringer] map[K]V

func (i Index[K, V]) Names() []string {
	var out []string
	for _, v := range i {
		out = append(out, v.String())
	}
	return out
}
