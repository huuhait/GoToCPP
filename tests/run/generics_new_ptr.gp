// phase: 07

// Pointers to type parameters: new(T), &v, *p, pointer receivers through *T, and pointers in slices.
package main

import "fmt"

type Point struct{ X, Y int }

func Ptr[T any](v T) *T { return &v }

func Deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

func Set[T any](p *T, v T) { *p = v }

func Swap[T any](a, b *T) { *a, *b = *b, *a }

func Alloc[T any](n int) []*T {
	out := make([]*T, n)
	for i := range out {
		out[i] = new(T)
	}
	return out
}

func Ptrs[T any](xs []T) []*T {
	out := make([]*T, len(xs))
	for i := range xs {
		out[i] = &xs[i]
	}
	return out
}

func Update[T any](xs []T, f func(*T)) {
	for i := range xs {
		f(&xs[i])
	}
}

func PP[T any](v T) **T {
	p := &v
	return &p
}

func main() {
	p := Ptr(42)
	q := Ptr("str")
	r := Ptr(Point{1, 2})
	fmt.Println(*p, *q, *r, r.X)
	*p += 1
	r.Y = 20
	fmt.Println(*p, *r, Deref(p, 0), Deref[int](nil, -1), Deref[Point](nil, Point{9, 9}))
	Set(q, "changed")
	a, b := 1, 2
	Swap(&a, &b)
	fmt.Println(*q, a, b)
	ps := Alloc[Point](2)
	ps[0].X = 7
	fmt.Println(*ps[0], *ps[1], len(Alloc[int](3)))
	xs := []int{1, 2, 3}
	for _, p := range Ptrs(xs) {
		*p *= 10
	}
	Update(xs, func(p *int) { *p++ })
	fmt.Println(xs)
	pts := []Point{{1, 1}, {2, 2}}
	Update(pts, func(p *Point) { p.X, p.Y = p.Y, p.X*10 })
	fmt.Println(pts)
	pp := PP("deep")
	**pp += "er"
	fmt.Println(**pp, *pp != nil)
}

// Output:
// 42 str {1 2} 1
// 43 {1 20} 43 -1 {9 9}
// changed 2 1
// {7 0} {0 0} 3
// [11 21 31]
// [{1 10} {2 20}]
// deeper true
