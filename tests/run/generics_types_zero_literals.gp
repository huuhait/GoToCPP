// phase: 07

// Zero values and composite literals of generic types: keyed, positional, nested, elided, pointers.
package main

import "fmt"

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Opt[T any] struct {
	v  T
	ok bool
}

type Grid[T any] struct {
	cells [2][2]T
	name  string
}

func main() {
	var p Pair[string, []int]
	fmt.Println(p.Key == "", p.Val == nil, p)
	q := Pair[int, string]{Key: 1}
	r := Pair[int, string]{2, "two"}
	fmt.Println(q, r, Pair[bool, float64]{})
	ps := []Pair[string, int]{{"a", 1}, {Key: "b"}, {}}
	fmt.Println(ps, len(ps))
	mp := map[string]Opt[int]{"x": {1, true}, "y": {}}
	fmt.Println(mp["x"], mp["y"], mp["z"].ok)
	pp := &Pair[int, int]{3, 4}
	pp.Val++
	fmt.Println(*pp, new(Pair[string, string]).Key == "")
	var g Grid[float64]
	g.cells[1][0] = 2.5
	g.name = "g"
	fmt.Printf("%v %+v\n", g, Grid[string]{name: "s"})
	arr := [3]Opt[string]{{"a", true}}
	fmt.Println(arr)
	var np *Pair[int, int]
	fmt.Println(np == nil, np)
	fmt.Printf("%v %+v %T\n", Opt[[]string]{}, Opt[*int]{}, Opt[map[string]bool]{})
}

// Output:
// true true { []}
// {1 } {2 two} {false 0}
// [{a 1} {b 0} { 0}] 3
// {1 true} {0 false} false
// {3 5} true
// {[[0 0] [2.5 0]] g} {cells:[[ ] [ ]] name:s}
// [{a true} { false} { false}]
// true <nil>
// {[] false} {v:<nil> ok:false} main.Opt[map[string]bool]
