// phase: 03

// Conversions where the types are not identical (step 3.9): between a named struct type and an
// identical anonymous one, implicit (a variable, an argument, a return value, a result of two,
// a literal element, a map key, ==, a switch case, a range target) and explicit (type B A,
// ignoring tags, (*B)(p) sharing the struct), between named and unnamed slices, arrays and maps
// (implicit both ways where Go assigns them), and package variables of named types.
package main

import "fmt"

type Point struct{ X, Y int }
type B Point
type Tagged struct {
	X int `json:"x"`
	Y int
}
type IDs []int
type Grid [2]int
type M map[string]int
type Pts []Point

var gpt Point = struct{ X, Y int }{1, 2}
var gids IDs = []int{9, 8}
var gg Grid = [2]int{3, 4}
var gc Celsius = 5
var gn Name = "pkg"

type Celsius float64
type Name string

func anon() struct{ X, Y int } { return struct{ X, Y int }{7, 8} }
func takeP(p Point) int        { return p.X*10 + p.Y }
func retP() Point              { return struct{ X, Y int }{5, 6} }
func two() (struct{ X, Y int }, []int) {
	return struct{ X, Y int }{3, 4}, []int{1}
}
func ints() []int        { return []int{1, 2, 3} }
func takeIDs(ids IDs) int { return len(ids) }
func takeInts(s []int) int {
	return len(s)
}

func main() {
	var p Point = anon()
	fmt.Println(p, takeP(anon()), retP(), gpt)
	b := B(p)
	b.X = 100
	fmt.Println(b, p, Point(b))
	t := Tagged(p)
	fmt.Println(t, Point(t))
	pb := (*B)(&p)
	pb.Y = 42
	fmt.Println(p.Y)
	var q Point
	var ids IDs
	q, ids = two()
	fmt.Println(q, len(ids))
	ps := []Point{struct{ X, Y int }{1, 1}, {2, 2}}
	fmt.Println(ps[1], ps[0] == struct{ X, Y int }{1, 1})
	m := map[Point]int{struct{ X, Y int }{1, 1}: 5}
	fmt.Println(m[struct{ X, Y int }{1, 1}])
	switch p {
	case anon():
		fmt.Println("anon")
	case struct{ X, Y int }{7, 42}:
		fmt.Println("matched")
	}
	var a struct{ X, Y int }
	for _, a = range []Point{{9, 9}} {
	}
	fmt.Println(a)
	ids = ints()
	fmt.Println(takeIDs(ints()), takeInts(ids), takeIDs([]int{1}))
	var ss [][]int = [][]int{ids}
	fmt.Println(len(ss[0]))
	g := Grid([2]int{1, 2})
	var arr [2]int = g
	fmt.Println(g[0], arr[1], g == [2]int{1, 2}, gg[1], gids[0], gc, gn)
	mm := M(map[string]int{"a": 1})
	var plain map[string]int = mm
	plain["b"] = 2
	fmt.Println(len(mm), mm["b"])
	pts := Pts{{1, 2}}
	pts = append(pts, Point{3, 4})
	fmt.Println(len(pts), pts[1].X)
	ptsInts := []Point(pts)
	fmt.Println(len(ptsInts))
	sl := g[:]
	fmt.Println(sl[0], len(sl))
}
