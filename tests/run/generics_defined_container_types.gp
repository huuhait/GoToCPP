// phase: 09

// Generic types defined over slices, maps, functions, channels and pointers, with methods of
// value and pointer receivers, composed in structs, converted to their underlying types, used as
// map keys and printed with fmt (Stringer methods on instances).
package main

import (
	"fmt"
	"slices"
	"strings"
)

type List[T any] []T

func (l List[T]) Len() int     { return len(l) }
func (l *List[T]) Push(v ...T) { *l = append(*l, v...) }
func (l *List[T]) Pop() (v T, ok bool) {
	if len(*l) == 0 {
		return
	}
	v, *l = (*l)[len(*l)-1], (*l)[:len(*l)-1]
	return v, true
}
func (l List[T]) Map(f func(T) T) List[T] {
	out := make(List[T], len(l))
	for i, v := range l {
		out[i] = f(v)
	}
	return out
}
func (l List[T]) String() string { return fmt.Sprintf("List%v", []T(l)) }

type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(v T) Set[T] { s[v] = struct{}{}; return s }
func (s Set[T]) Has(v T) bool   { _, ok := s[v]; return ok }
func (s Set[T]) Union(o Set[T]) Set[T] {
	r := Set[T]{}
	for k := range s {
		r.Add(k)
	}
	for k := range o {
		r.Add(k)
	}
	return r
}

type Pred[T any] func(T) bool

func (p Pred[T]) Not() Pred[T]          { return func(v T) bool { return !p(v) } }
func (p Pred[T]) And(q Pred[T]) Pred[T] { return func(v T) bool { return p(v) && q(v) } }
func (p Pred[T]) Filter(xs []T) (out []T) {
	for _, x := range xs {
		if p(x) {
			out = append(out, x)
		}
	}
	return
}

type Pipe[T any] chan T

func (p Pipe[T]) Send(vs ...T) Pipe[T] {
	for _, v := range vs {
		p <- v
	}
	return p
}
func (p Pipe[T]) Drain() (out []T) {
	close(p)
	for v := range p {
		out = append(out, v)
	}
	return
}

type Ref[T any] *T

type Matrix[T ~int | ~float64] [2][2]T

func (m Matrix[T]) Mul(o Matrix[T]) (r Matrix[T]) {
	for i := range 2 {
		for j := range 2 {
			for k := range 2 {
				r[i][j] += m[i][k] * o[k][j]
			}
		}
	}
	return
}

type Holder struct {
	Names List[string]
	Tags  Set[string]
	Keep  Pred[int]
}

type Key struct{ L [2]string }

func main() {
	var l List[int]
	l.Push(1, 2, 3)
	v, ok := l.Pop()
	fmt.Println(l, l.Len(), v, ok, l.Map(func(i int) int { return i * 10 }), []int(l))
	fmt.Printf("%v %s %d %T\n", l, l, l, l)
	var e List[string]
	_, ok = e.Pop()
	fmt.Println(ok, e, e == nil, len(e))
	a := Set[string]{}.Add("x").Add("y")
	b := Set[string]{"z": {}}
	u := a.Union(b)
	fmt.Println(len(u), u.Has("z"), u.Has("q"), u)
	even := Pred[int](func(i int) bool { return i%2 == 0 })
	big := Pred[int](func(i int) bool { return i > 3 })
	fmt.Println(even.Filter([]int{1, 2, 3, 4, 5, 6}), even.Not().Filter([]int{1, 2, 3}), even.And(big).Filter([]int{2, 4, 6, 7}))
	fmt.Println(Pipe[string](make(chan string, 3)).Send("a", "b").Drain())
	x := 5
	var r Ref[int] = &x
	*r = 6
	fmt.Println(x, *r)
	m := Matrix[int]{{1, 1}, {1, 0}}
	p := m
	for range 6 {
		p = p.Mul(m)
	}
	fmt.Println(p, Matrix[float64]{{0.5, 0}, {0, 2}}.Mul(Matrix[float64]{{2, 1}, {1, 0.5}}))
	h := Holder{Names: List[string]{"n1"}, Tags: Set[string]{"t": {}}, Keep: even}
	h.Names.Push("n2")
	fmt.Printf("%v %+v\n", h.Names, h.Names)
	fmt.Println(h.Keep(2), strings.Join(h.Names, "+"), len(h.Tags))
	seen := map[Key]List[int]{}
	seen[Key{[2]string{"a", "b"}}] = List[int]{1}
	seen[Key{[2]string{"a", "b"}}] = append(seen[Key{[2]string{"a", "b"}}], 2)
	fmt.Println(seen, len(seen))
	ls := []List[int]{{3, 1}, {2}, nil}
	slices.SortFunc(ls, func(a, b List[int]) int { return a.Len() - b.Len() })
	fmt.Println(ls)
}

// Output:
// List[1 2] 2 3 true List[10 20] [1 2]
// List[1 2] List[1 2] [1 2] main.List[int]
// false List[] true 0
// 3 true false map[x:{} y:{} z:{}]
// [2 4 6] [1 3] [4 6]
// [a b]
// 6 6
// [[21 13] [13 8]] [[1 0.5] [2 1]]
// List[n1 n2] List[n1 n2]
// true n1+n2 1
// map[{[a b]}:List[1 2]] 1
// [List[] List[2] List[3 1]]
