// phase: 05

// Unnamed struct and interface types next to named ones: fields, results, parameters, type switches, embedding.
package main

import "fmt"

type Point struct{ X, Y int }
type Shape interface{ Area() int }
type Rect struct{ Min, Max Point }

func (r Rect) Area() int { return (r.Max.X - r.Min.X) * (r.Max.Y - r.Min.Y) }

type Holder struct {
	Pos  struct{ P Point; Tag string }
	Any  interface{ Area() int }
	Fn   func(struct{ A, B Point }) interface{ Bounds() Rect }
	List []struct {
		S Shape
		N int
	}
}

type Boxed struct{ r Rect }

func (b Boxed) Bounds() Rect { return b.r }

func mk(a, b Point) interface{ Bounds() Rect } { return Boxed{Rect{a, b}} }

func anonRet() struct {
	Pt Point
	Sh Shape
} {
	return struct {
		Pt Point
		Sh Shape
	}{Point{1, 2}, Rect{Point{0, 0}, Point{2, 3}}}
}

type Named interface {
	interface{ Area() int }
	Name() string
}
type NS struct{ Rect }

func (NS) Name() string { return "ns" }

func main() {
	h := Holder{}
	h.Pos.P = Point{3, 4}
	h.Pos.Tag = "t"
	h.Any = Rect{Point{0, 0}, Point{4, 5}}
	h.Fn = func(x struct{ A, B Point }) interface{ Bounds() Rect } { return mk(x.A, x.B) }
	h.List = append(h.List, struct {
		S Shape
		N int
	}{Rect{Point{1, 1}, Point{2, 2}}, 7})
	fmt.Println(h.Pos, h.Any.Area(), h.Fn(struct{ A, B Point }{Point{0, 0}, Point{9, 9}}).Bounds(), h.List[0].S.Area(), h.List[0].N)
	r := anonRet()
	fmt.Println(r, r.Sh.Area())
	var a any = r
	switch v := a.(type) {
	case struct {
		Pt Point
		Sh Shape
	}:
		fmt.Println("anon match", v.Pt)
	default:
		fmt.Println("no")
	}
	_, ok := a.(struct{ Pt Point })
	fmt.Println(ok)
	fmt.Printf("%T\n%T\n", a, h.Fn)
	var n Named = NS{Rect{Point{0, 0}, Point{1, 1}}}
	var ar interface{ Area() int } = n
	fmt.Println(n.Name(), ar.Area())
	_, isNamed := ar.(Named)
	_, isBounds := ar.(interface{ Bounds() Rect })
	fmt.Println(isNamed, isBounds)
	m := map[string]interface{ Area() int }{"a": Rect{Point{0, 0}, Point{1, 2}}, "b": NS{}}
	fmt.Println(m["a"].Area(), m["b"].Area(), len(m))
	x, y := struct{ P Point }{Point{1, 2}}, struct{ P Point }{Point{1, 2}}
	fmt.Println(x == y)
	var arr [2]interface{ Area() int }
	fmt.Println(arr[0] == nil, len(arr))
}
