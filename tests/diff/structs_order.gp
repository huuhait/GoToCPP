// phase: 03

// Go's order of evaluation with structs: the values of a keyed literal run in source order
// (C++ initializes the fields in declaration order), a field of a package struct or through a
// pointer is read after the calls of its statement, the pointer of a target p.f is evaluated
// before the values (the store goes through the old p), and fmt's arguments follow gc's copies:
// a struct that is not addressable is copied in its place unless gc converts it by value.
package main

import "fmt"

type Pt struct{ a, b int }

type One struct{ v int }

type Pair struct {
	name string
	p    Pt
}

var g = Pt{1, 2}
var cur = &g
var other = Pt{10, 20}
var n = 0

func tick(tag string) int {
	n++
	fmt.Print(tag, n, " ")
	g.a += 100
	if n%2 == 0 {
		cur = &other
	} else {
		cur = &g
	}
	return n
}

func label(n int) string {
	return string(rune('a' + n))
}

func main() {
	fmt.Println(Pt{b: tick("b"), a: tick("a")})
	fmt.Println(Pair{p: Pt{b: tick("pb"), a: g.a}, name: label(tick("name"))})
	fmt.Println(g.a, tick("x"), cur.b, tick("y"), cur.b)
	fmt.Println(Pt{g.a, 0}, tick("z"))
	fmt.Println(One{g.a}, tick("w"))
	fmt.Println(Pt{g.a, 0} == Pt{g.a, 0}, tick("v"))

	q := cur
	cur, cur.a = &other, tick("store")
	fmt.Println(*q, g, other)
	cur.b += tick("op")
	fmt.Println(g, other)

	pts := []Pt{{1, 1}, {2, 2}}
	i := 0
	i, pts[i].b = 1, tick("idx")
	fmt.Println(i, pts[0], pts[1])
	for _, pts[i].a = range []int{tick("r1"), tick("r2")} {
	}
	fmt.Println(pts[0], pts[1])
	pts[0], pts[1] = pts[1], pts[0]
	fmt.Println(pts[0], pts[1])
}
