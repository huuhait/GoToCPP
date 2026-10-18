// phase: 07

// Structs as type arguments: copies are independent, == compares fields, nested and embedded fields work.
package main

import "fmt"

type Point struct{ X, Y int }

type Line struct {
	A, B Point
	Name string
}

type Base struct{ ID int }
type Item struct {
	Base
	Tags [2]string
	Opt  *int
}

func Copy[T any](v T) (T, T) { return v, v }

func Modify[T any](v T, f func(*T)) T {
	f(&v)
	return v
}

func Eq[T comparable](a, b T) bool { return a == b }

func Slice[T any](n int, v T) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func Index[T comparable](xs []T, v T) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

func MapOf[K comparable, V any](k K, v V) map[K]V { return map[K]V{k: v} }

func main() {
	a, b := Copy(Point{1, 2})
	b.X = 100
	fmt.Println(a, b)
	l := Modify(Line{Point{1, 1}, Point{2, 2}, "l"}, func(l *Line) { l.B.X = 9; l.Name += "!" })
	fmt.Println(l, l.A, l.B)
	n := 5
	i1 := Item{Base{1}, [2]string{"a", "b"}, &n}
	i2 := Item{Base{1}, [2]string{"a", "b"}, &n}
	fmt.Println(Eq(i1, i2), Eq(i1, Item{}), Eq(Line{}, Line{}), Eq(struct{}{}, struct{}{}))
	i3, i4 := Copy(i1)
	i4.Tags[0] = "z"
	i4.ID = 2
	fmt.Println(i3.Tags, i4.Tags, i3.ID, i4.ID, i3.Opt == i4.Opt)
	pts := Slice(3, Point{7, 8})
	pts[0].X = 0
	fmt.Println(pts, Index(pts, Point{7, 8}), Index(pts, Point{}))
	m := MapOf(Point{1, 2}, Line{Name: "m"})
	fmt.Println(m, len(MapOf(Base{1}, 1)))
	fmt.Printf("%v %+v %T\n", i1.Base, l.A, Modify(Base{}, func(*Base) {}))
}

// Output:
// {1 2} {100 2}
// {{1 1} {9 2} l!} {1 1} {9 2}
// true false true true
// [a b] [z b] 1 2 true
// [{0 8} {7 8} {7 8}] 1 -1
// map[{1 2}:{{0 0} {0 0} m}] 1
// {1} {X:1 Y:1} main.Base
