// phase: 07

// Anonymous struct types and composite literals that mention type parameters.
package main

import "fmt"

type Named struct {
	Label string
	N     int
}

func Wrap[T any](v T) struct {
	Val T
	Ok  bool
} {
	return struct {
		Val T
		Ok  bool
	}{v, true}
}

func Pairs[K comparable, V any](keys []K, vals []V) []struct {
	K K
	V V
} {
	out := make([]struct {
		K K
		V V
	}, len(keys))
	for i := range keys {
		out[i].K = keys[i]
		out[i].V = vals[i]
	}
	return out
}

func Table[T any](rows ...[]T) map[int][]T {
	m := map[int][]T{}
	for i, r := range rows {
		m[i] = r
	}
	return m
}

func Grid[T any](w, h int, v T) [][]T {
	g := make([][]T, h)
	for y := range g {
		g[y] = make([]T, w)
		for x := range g[y] {
			g[y][x] = v
		}
	}
	return g
}

func Lit[T any](a, b, c T) ([3]T, []T, map[string]T, *[]T) {
	return [3]T{a, b, c}, []T{c, b, a}, map[string]T{"a": a, "c": c}, &[]T{b}
}

func Indexed[T any](v T) [5]T { return [5]T{1: v, 3: v} }

func main() {
	w := Wrap(5)
	x := Wrap("s")
	y := Wrap(Named{"n", 1})
	fmt.Println(w, x, y, w.Val+1, x.Val+"!", y.Val.Label, y.Ok)
	fmt.Printf("%v %+v %T\n", w, y, x)
	ps := Pairs([]string{"a", "b"}, []float64{1.5, 2.5})
	fmt.Println(ps, ps[1].K, ps[0].V)
	fmt.Println(Table([]int{1}, []int{2, 3}), Table[string]())
	fmt.Println(Grid(3, 2, '.'), Grid(2, 2, Named{"z", 0}), len(Grid(0, 3, 1.5)))
	a, s, m, p := Lit(1, 2, 3)
	fmt.Println(a, s, m, *p)
	fmt.Println(Indexed("x"), Indexed(Named{"a", 1})[3], Indexed(1.5))
}

// Output:
// {5 true} {s true} {{n 1} true} 6 s! n true
// {5 true} {Val:{Label:n N:1} Ok:true} struct { Val string; Ok bool }
// [{a 1.5} {b 2.5}] b 1.5
// map[0:[1] 1:[2 3]] map[]
// [[46 46 46] [46 46 46]] [[{z 0} {z 0}] [{z 0} {z 0}]] 3
// [1 2 3] [3 2 1] map[a:1 c:3] [2]
// [ x  x ] {a 1} [0 1.5 0 1.5 0]
