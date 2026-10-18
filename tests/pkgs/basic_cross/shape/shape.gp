package shape

import (
	"fmt"

	"example.com/shop/geom"
)

type Shape interface {
	Area() float64
	Name() string
}

type Rect struct{ W, H float64 }

func (r Rect) Area() float64 { return r.W * r.H }
func (r Rect) Name() string  { return "rect" }

type Circle struct{ R float64 }

func (c *Circle) Area() float64 { return geom.Pi * c.R * c.R }
func (c *Circle) Name() string  { return "circle" }

type Item struct{ Label string }

func (i Item) Describe() string { return "shape item " + i.Label }

type Describer interface{ Describe() string }

func Total(ss []Shape) float64 {
	t := 0.0
	for _, s := range ss {
		t += s.Area()
	}
	return t
}

func Show(s Shape) { fmt.Println(s.Name(), s.Area()) }

func init() { fmt.Println("init shape") }
