// Structs: fields, tags, embedding and promotion, a zero-size field (empty struct and [0]int),
// anonymous structs, keyed and positional composite literals, pointers to structs, new(T), and
// the value-copy semantics of assignment.
package main

type Point struct {
	X, Y int
}

type Empty struct{}

type Tagged struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
}

type Base struct {
	ID int
}

type Embedded struct {
	Base
	Name string
}

type WithZero struct {
	A int
	e Empty
	Z [0]int
}

func main() {
	p := Point{1, 2}
	q := Point{X: 3, Y: 4}
	println(p.X, p.Y, q.X, q.Y)
	p.X = 10
	pp := &p
	pp.Y = 20
	println(p.X, p.Y, (*pp).X)
	np := new(Point)
	println(np.X, np.Y, np == nil)
	cp := p
	cp.X = 99
	println(p.X, cp.X)
	e := Embedded{Base{7}, "n"}
	println(e.ID, e.Name)
	e.ID = 8
	println(e.Base.ID)
	var z WithZero
	z.A = 1
	println(z.A)
	anon := struct {
		N int
		M string
	}{5, "m"}
	println(anon.N, anon.M)
	t := Tagged{A: 1}
	println(t.A, t.B == "")
	var pt *Point
	println(pt == nil)
	pt = &Point{9, 8}
	println(pt.X)
}
