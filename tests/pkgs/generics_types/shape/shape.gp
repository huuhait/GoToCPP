package shape

import "fmt"

type Point struct{ X, Y int }

func (p Point) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

type hidden struct{ n int }

func (h hidden) String() string { return fmt.Sprint("hidden", h.n) }

func Make(n int) any { return hidden{n} }

type Kind int

const (
	Circle Kind = iota
	Square
)

func (k Kind) String() string { return [...]string{"circle", "square"}[k] }

type Shape interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

type Rect struct{ W, H float64 }

func (r *Rect) Area() float64 { return r.W * r.H }
