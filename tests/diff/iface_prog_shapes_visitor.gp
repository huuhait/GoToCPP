// phase: 05

// Shapes, a visitor, a sorter over an interface and bound/expression method values.
package main

import "fmt"

const Pi = 3.14159265358979

type Shape interface {
	Area() float64
	Perimeter() float64
}

type Named interface{ Name() string }

type Visitor interface {
	VisitCircle(*Circle)
	VisitRect(*Rect)
}

type Circle struct{ R float64 }
type Rect struct{ W, H float64 }
type Square struct{ Rect }

func (c *Circle) Area() float64      { return Pi * c.R * c.R }
func (c *Circle) Perimeter() float64 { return 2 * Pi * c.R }
func (c *Circle) Name() string       { return "circle" }
func (r *Rect) Area() float64        { return r.W * r.H }
func (r *Rect) Perimeter() float64   { return 2 * (r.W + r.H) }
func (r *Rect) Name() string         { return "rect" }
func NewSquare(s float64) *Square    { return &Square{Rect{s, s}} }
func (s *Square) Name() string       { return "square/" + s.Rect.Name() }

type Acceptor interface{ Accept(Visitor) }

func (c *Circle) Accept(v Visitor) { v.VisitCircle(c) }
func (r *Rect) Accept(v Visitor)   { v.VisitRect(r) }

type counter struct{ circles, rects int }

func (c *counter) VisitCircle(*Circle) { c.circles++ }
func (c *counter) VisitRect(*Rect)     { c.rects++ }

type byArea []Shape

func (b byArea) Len() int           { return len(b) }
func (b byArea) Less(i, j int) bool { return b[i].Area() < b[j].Area() }
func (b byArea) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type sorter interface {
	Len() int
	Less(i, j int) bool
	Swap(i, j int)
}

func bubble(s sorter) {
	for i := 0; i < s.Len(); i++ {
		for j := 0; j+1 < s.Len()-i; j++ {
			if s.Less(j+1, j) {
				s.Swap(j, j+1)
			}
		}
	}
}

func main() {
	shapes := []Shape{&Circle{2}, &Rect{3, 4}, NewSquare(2), &Circle{0.5}}
	bubble(byArea(shapes))
	c := &counter{}
	for _, s := range shapes {
		name := "?"
		if n, ok := s.(Named); ok {
			name = n.Name()
		}
		fmt.Printf("%-10s %8.3f %8.3f %T\n", name, s.Area(), s.Perimeter(), s)
		if a, ok := s.(Acceptor); ok {
			a.Accept(c)
		}
	}
	fmt.Println(*c)
	var total float64
	for _, s := range shapes {
		switch v := s.(type) {
		case *Circle:
			total += v.R
		case *Square:
			total += v.W * 10
		case *Rect:
			total += v.W
		}
	}
	fmt.Println(total)
	f := shapes[0].Area
	g := Shape.Perimeter
	fmt.Printf("%.2f %.2f\n", f(), g(shapes[1]))
}
