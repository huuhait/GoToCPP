// Named types that aren't structs (Phase 03 step 3.9): named types over numbers, bools,
// strings, slices, arrays, maps and pointers (gp::newtype), also local ones and named types of
// named types; zero values; every operator of the underlying type giving the named type
// (small integers wrap, division and shifts with Go's rules, op=, ++), untyped constants;
// conversions both ways and between named types; the results that give the named type
// (s[a:b], append, make, +, literals); implicit conversions where Go assigns a value of an
// unnamed type to a named one (assignment, argument, return, literal element, map key, ==,
// switch case, range target, several results) and between struct types (named and anonymous,
// type B A, (*B)(p)); switch on a named integer (its value, folded constants), a named string
// and a named bool; map keys of named types; package variables (constinit); fmt prints the
// value and println too.
package main

import "fmt"

type Celsius float64

type Fahrenheit float64

type Small uint8

type Color int

const (
	Red Color = iota
	Green
	Blue
)

type Name string

type IDs []int

type Grid [3]int

type Ages map[Name]int

type Ok bool

type Temp Celsius // over float64, like Celsius

// Node holds a Link by value: Link is defined before Node, and its base gp::ptr<Node> takes the
// incomplete Node.
type Node struct {
	val  int
	next Link
}

type Link *Node

type Point struct{ X, Y int }

type Moved Point // a struct of its own, with Point's fields

var gc Celsius = 21.5

var gn Name = "pkg"

var gg = Grid{7, 8, 9}

var gids IDs = []int{4, 5}

func toF(c Celsius) Fahrenheit {
	return Fahrenheit(c*9/5 + 32)
}

func colorName(c Color) string {
	switch c {
	case Red:
		return "red"
	case Green, Blue:
		return "green or blue"
	}
	return "?"
}

func ints() []int {
	return []int{1, 2, 3}
}

func count(ids IDs) int {
	return len(ids)
}

func sum(s []int) (t int) {
	for _, v := range s {
		t += v
	}
	return
}

func anon() struct{ X, Y int } {
	return struct{ X, Y int }{3, 4}
}

func point() Point {
	return struct{ X, Y int }{5, 6}
}

func two() (IDs, struct{ X, Y int }) {
	return []int{9}, struct{ X, Y int }{1, 1}
}

func numbers() {
	c := Celsius(36.6)
	var zero Celsius
	fmt.Println(c, toF(c), -c, c > 30, zero == 0, Temp(c)+0.5, float64(c), int(c))
	s := Small(200)
	s += 100
	s++
	k := 3
	fmt.Println(s, s<<2, s>>1, s<<k, ^s, s&^3, s/7, s%7, s*s, Small(k)-s)
	var ok Ok = c > 30
	fmt.Println(ok, !ok, ok && c < 40, ok || false)
	if ok {
		fmt.Println("ok")
	}
	for i := range Color(3) {
		fmt.Println(i, colorName(i))
	}
	switch b := Blue; b {
	case Red:
		fmt.Println("r")
	case Blue:
		fmt.Println("b")
		fallthrough
	case Green:
		fmt.Println("g")
	}
	switch ok {
	case false:
		fmt.Println("not ok")
	case true:
		fmt.Println("ok again")
	}
	fmt.Printf("%v %d %t %f\n", c, s, ok, toF(zero))
	println(c, s, ok)
}

func composites() {
	n := Name("gopher")
	n2 := n[1:4] + "!"
	fmt.Println(n2, len(n2), n2 < n, n[0], string(n[:2]), []byte(n)[1], Name([]byte{'h', 'i'}))
	for i, r := range n[:2] {
		fmt.Println(i, r)
	}
	switch n2 {
	case "x":
		fmt.Println("x")
	case "oph" + "!":
		fmt.Println("oph!")
	}
	var ids IDs
	fmt.Println(ids == nil, count(ids), count(nil))
	ids = append(ids, 1, 2, 3)
	ids = ids[1:]
	ids2 := make(IDs, 2)
	ids2[1] = 7
	fmt.Println(len(ids), ids[0], sum(ids), sum(ids2), count(IDs{5}))
	ids = ints()
	var plain []int = ids
	fmt.Println(count(ints()), sum(plain), len([]int(ids)), len(IDs(plain)))
	g := Grid{1, 2, 3}
	g[1] = 20
	part := g[:2]
	fmt.Println(g[1], g == Grid{1, 20, 3}, g == [3]int{1, 20, 3}, len(part), gg[2])
	for i, v := range g {
		fmt.Println(i, v)
	}
	ages := Ages{"ann": 30}
	ages["bob"] = 25
	ages["ann"]++
	age, found := ages["bob"]
	fmt.Println(len(ages), ages["ann"], age, found, ages[n])
	byColor := map[Color]Celsius{Red: 1.5}
	byColor[Blue] += 2
	fmt.Println(byColor[Blue], byColor[Red], len(byColor))
	var l Link = &Node{val: 1}
	l.next = &Node{val: 2}
	var p *Node = l
	fmt.Println(l.val, l.next.val, p.val, l != nil, (*Node)(l) == p)
	type Local int
	type Pair struct{ a, b Local }
	pr := Pair{1, 2}
	pr.a += pr.b * 10
	fmt.Println(pr, Local(7)/2)
}

func conversions() {
	var p Point = anon()
	fmt.Println(p, point(), Moved(p), Point(Moved(p)))
	pm := (*Moved)(&p)
	pm.Y = 42
	fmt.Println(p.Y, p == struct{ X, Y int }{3, 42})
	var ids IDs
	ids, p = two()
	fmt.Println(len(ids), p)
	ps := []Point{struct{ X, Y int }{1, 2}}
	m := map[Point]int{struct{ X, Y int }{1, 2}: 3}
	fmt.Println(ps[0], m[struct{ X, Y int }{1, 2}])
	switch p {
	case anon():
		fmt.Println("anon")
	case struct{ X, Y int }{1, 1}:
		fmt.Println("one one")
	}
	var a struct{ X, Y int }
	for _, a = range ps {
	}
	fmt.Println(a)
}

func main() {
	numbers()
	composites()
	conversions()
	fmt.Println(gc, gn, gg[0], gids[1])
}
