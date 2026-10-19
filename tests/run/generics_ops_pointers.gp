// phase: 07

// Pointers to type parameters: new, address-of, dereference, nil, pointers to fields and elements.
package main

import "fmt"

type Pt struct{ X, Y int }

func Ptr[T any](v T) *T { return &v }
func Set[T any](p *T, v T) { *p = v }
func Get[T any](p *T) T { return *p }
func GetOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}
func Swap[T any](a, b *T) { *a, *b = *b, *a }
func Elem[T any](s []T, i int) *T { return &s[i] }
func Alias[T any](v T) (*T, *T) {
	p := &v
	return p, p
}
func Chain[T any](v T) **T {
	p := &v
	return &p
}
func Copy[T any](p *T) *T { v := *p; return &v }
func SamePtr[T any](a, b *T) bool { return a == b }
func Update[T any](p *T, f func(T) T) { *p = f(*p) }
func NewN[T any](n int) []*T {
	out := make([]*T, n)
	for i := range out {
		out[i] = new(T)
	}
	return out
}

func main() {
	p := Ptr(5)
	q := Ptr("s")
	r := Ptr(Pt{1, 2})
	Set(p, 6)
	Set(q, *q+"t")
	r.X = 10
	fmt.Println(*p, *q, *r, Get(p), Get(q), Get(r))
	var np *int
	fmt.Println(GetOr(np, 42), GetOr(p, 42), GetOr[Pt](nil, Pt{9, 9}))
	a, b := 1, 2
	Swap(&a, &b)
	x, y := Pt{1, 1}, Pt{2, 2}
	Swap(&x, &y)
	fmt.Println(a, b, x, y)
	s := []Pt{{1, 2}, {3, 4}}
	e := Elem(s, 1)
	e.Y = 40
	Set(Elem(s, 0), Pt{7, 7})
	fmt.Println(s)
	a1, a2 := Alias(1.5)
	*a1 = 2.5
	fmt.Println(*a2, a1 == a2, **Chain("deep"))
	c := Copy(r)
	c.X = 99
	fmt.Println(r.X, c.X, SamePtr(r, r), SamePtr(r, c), SamePtr(np, np))
	Update(p, func(v int) int { return v * 7 })
	Update(q, func(v string) string { return v + v })
	fmt.Println(*p, *q)
	ptrs := NewN[Pt](2)
	ptrs[0].X = 5
	fmt.Println(*ptrs[0], *ptrs[1], ptrs[0] != ptrs[1])
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(Get(np))
}

// Output:
// 6 st {10 2} 6 st {10 2}
// 42 6 {9 9}
// 2 1 {2 2} {1 1}
// [{7 7} {3 40}]
// 2.5 true deep
// 10 99 true false true
// 42 stst
// {5 0} {0 0} true
// recovered: runtime error: invalid memory address or nil pointer dereference
