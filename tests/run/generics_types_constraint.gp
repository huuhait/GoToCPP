// phase: 07

// Type parameters of types with constraints: type sets, methods through constraints, comparable.
package main

import "fmt"

type Number interface{ ~int | ~int8 | ~float64 }

type Stringer interface{ String() string }

type Acc[T Number] struct{ total T }

func (a *Acc[T]) Add(v T)  { a.total += v }
func (a Acc[T]) Total() T  { return a.total }
func (a Acc[T]) Half() T   { return a.total / 2 }

type Labeled[T Stringer] struct {
	items []T
	sep   string
}

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

type Index[K comparable, V any] struct{ m map[K]V }

func NewIndex[K comparable, V any]() *Index[K, V] { return &Index[K, V]{m: map[K]V{}} }

func (i *Index[K, V]) Put(k K, v V) bool {
	_, dup := i.m[k]
	i.m[k] = v
	return !dup
}

func (i *Index[K, V]) Get(k K) (V, bool) { v, ok := i.m[k]; return v, ok }

type ID int

func (i ID) String() string { return fmt.Sprintf("#%d", int(i)) }

type Celsius float64

func main() {
	var a Acc[int8]
	a.Add(100)
	a.Add(27)
	a.Add(1)
	fmt.Println(a.Total(), a.Half())
	f := Acc[Celsius]{}
	f.Add(1.5)
	f.Add(2)
	fmt.Println(f.Total(), f.Half())
	l := Labeled[ID]{[]ID{1, 2, 3}, ","}
	fmt.Println(l.Join())
	ix := NewIndex[string, []int]()
	fmt.Println(ix.Put("a", []int{1}), ix.Put("a", nil))
	fmt.Println(ix.Get("a"))
	fmt.Println(ix.Get("b"))
	fmt.Printf("%T %T %T\n", a, f, l)
}

// Output:
// -128 -64
// 3.5 1.75
// #1,#2,#3
// true false
// [] true
// [] false
// main.Acc[int8] main.Acc[main.Celsius] main.Labeled[main.ID]
