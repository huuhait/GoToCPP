// phase: 05

// Structs with embedded fields are comparable when the embedded type is, and can be map keys;
// an embedded pointer compares by address.
package main

import "fmt"

type Point struct{ X, Y int }
type Named struct {
	Point
	Name string
}
type Ref struct {
	*Point
	K int
}
type Tagged struct {
	Named
	Tag string
}

func main() {
	a := Named{Point{1, 2}, "a"}
	b := Named{Point{1, 2}, "a"}
	c := Named{Point{1, 3}, "a"}
	fmt.Println(a == b, a == c, a != c, a.Point == b.Point, a.Point == c.Point)
	m := map[Named]int{a: 1, c: 2}
	m[b]++
	fmt.Println(len(m), m[a], m[Named{Point{1, 3}, "a"}], m[Named{}])
	t1 := Tagged{a, "t"}
	t2 := Tagged{b, "t"}
	mt := map[Tagged][]string{}
	mt[t1] = append(mt[t1], "one")
	mt[t2] = append(mt[t2], "two")
	fmt.Println(t1 == t2, len(mt), mt[t1])
	pt := &Point{1, 2}
	r1, r2, r3 := Ref{pt, 1}, Ref{pt, 1}, Ref{&Point{1, 2}, 1}
	fmt.Println(r1 == r2, r1 == r3, r1.Point == r3.Point, *r1.Point == *r3.Point)
	mr := map[Ref]string{r1: "r1"}
	mr[r2] += "+r2"
	mr[r3] = "r3"
	fmt.Println(len(mr), mr[r1])
	var e1, e2 Ref
	fmt.Println(e1 == e2, e1.Point == nil)
	arr1 := [2]Named{a, c}
	arr2 := [2]Named{b, c}
	fmt.Println(arr1 == arr2)
}

// Output:
// true false true true false
// 2 2 2 0
// true 1 [one two]
// true false false true
// 2 r1+r2
// true true
// true
